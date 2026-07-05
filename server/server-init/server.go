// Package server handles startup, configuration, and graceful shutdown of the
// Nimbus API. InitServer is the single entry point called from main.
package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/nimbus/api/db/postgres"
	redisdb "github.com/nimbus/api/db/redis"
	s3db "github.com/nimbus/api/db/s3"
	"github.com/nimbus/api/middleware/bodylimit"
	"github.com/nimbus/api/middleware/ratelimit"
	"github.com/nimbus/api/routes"
	"github.com/nimbus/api/utils"
	"gorm.io/gorm"
)

// S3 and DB are package-level singletons shared across all request handlers.
var S3 *s3.Client
var DB *gorm.DB

// InitServer bootstraps the entire API:
//  1. Reads required environment variables
//  2. Creates the Gin router with logging, recovery, and CORS middleware
//  3. Connects to S3 and PostgreSQL
//  4. Registers all route groups
//  5. Starts the HTTP server in a background goroutine
//  6. Waits for SIGINT/SIGTERM, then shuts down cleanly within 10 seconds
func InitServer() error {
	bucket, err := utils.GetEnv("S3_BUCKET")
	if err != nil {
		return err
	}

	// gin.New() gives us a blank router — we add Logger and Recovery manually
	// so we keep full control over middleware order.
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Cap request body size so a client can't force the server to buffer an
	// arbitrarily large body. File uploads bypass this (they go straight to S3
	// via presigned URLs), so a small JSON-sized limit is safe for every route.
	r.Use(bodylimit.Middleware(bodylimit.DefaultMaxBytes))

	// Trust ALB and private RFC-1918 ranges so X-Forwarded-For gives real client IPs.
	// In LOCAL_DEV mode trust all proxies since there's no ALB in docker-compose.
	localDev, _ := utils.GetEnv("LOCAL_DEV")
	if localDev == "true" {
		r.SetTrustedProxies(nil)
	} else {
		r.SetTrustedProxies([]string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})
	}

	// CORS: allow all origins by default; restrict to a comma-separated list
	// when CORS_ORIGINS is set (e.g. "https://app.example.com").
	corsOrigins, _ := utils.GetEnv("CORS_ORIGINS")
	origins := []string{"*"}
	if corsOrigins != "" {
		origins = strings.Split(corsOrigins, ",")
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: corsOrigins != "",
		MaxAge:           12 * time.Hour,
	}))

	ctx := context.Background()

	region, err := utils.GetEnv("AWS_REGION")
	if err != nil {
		return err
	}

	// Connect to S3 (or LocalStack when S3_ENDPOINT is set in the env).
	s3Client, err := s3db.Connect(ctx, region)
	if err != nil {
		return err
	}

	S3 = s3Client
	if S3 == nil {
		return fmt.Errorf("failed to connect to S3")
	}

	// Bundle the S3 client + bucket name into a single config struct that
	// every handler receives so they never read global state directly.
	config := s3db.Config{
		Client: S3,
		Bucket: bucket,
	}

	// Connect to PostgreSQL and auto-migrate all models.
	DB, err = postgres.Connect()
	if err != nil {
		return err
	}
	if DB == nil {
		return fmt.Errorf("failed to connect to PostgreSQL")
	}

	// Build the auth rate limiter: 5 attempts / 15 min per key. Use Redis when
	// REDIS_ADDR is configured so the limit is shared across all instances behind
	// the load balancer and survives restarts; otherwise fall back to a
	// per-process in-memory limiter (fine for single-instance / local dev).
	redisClient, err := redisdb.Connect(ctx)
	if err != nil {
		return err
	}
	var authLimiter *ratelimit.Limiter
	if redisClient != nil {
		authLimiter = ratelimit.NewWithRedis(redisClient, 5, 15*time.Minute)
		log.Println("Rate limiter: using Redis (shared across instances)")
	} else {
		authLimiter = ratelimit.New(5, 15*time.Minute)
		log.Println("Rate limiter: using in-memory store (REDIS_ADDR not set)")
	}

	// Register all route groups (files, boxes, folders, users).
	routes.InitFileRoutes(r, config, DB)
	routes.InitBoxRoutes(r, config, DB)
	routes.InitFolderRoutes(r, config, DB)
	routes.InitUserRoutes(r, DB, S3, authLimiter)

	// /health checks both the database and S3 so the ALB only routes traffic to
	// a fully operational instance. Returns 503 if either dependency is down.
	r.GET("/health", func(c *gin.Context) {
		// Ping the database.
		sqlDB, err := DB.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "reason": "database unavailable"})
			return
		}

		// Verify S3 is reachable by listing objects in the bucket (max 1 result).
		hCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		maxKeys := int32(1)
		if _, err := S3.ListObjectsV2(hCtx, &s3.ListObjectsV2Input{
			Bucket:  &bucket,
			MaxKeys: &maxKeys,
		}); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "reason": "storage unavailable"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Configure HTTP server timeouts.
	// WriteTimeout is generous (300s) to accommodate large file presign operations.
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the server in a goroutine so we can block on the signal channel below.
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Block until we receive SIGINT (Ctrl-C) or SIGTERM (Docker/ECS stop).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Give in-flight requests up to 10 seconds to finish before we close.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Close the database connection pool cleanly.
	if sqlDB, err := DB.DB(); err == nil {
		_ = sqlDB.Close()
	}

	log.Println("Server exited cleanly")
	return nil
}
