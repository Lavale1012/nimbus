// Package routes wires HTTP verbs + paths to handler functions.
// Each Init* function is called once at startup from server-init/server.go.
package routes

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/nimbus/api/handlers/user"
	"github.com/nimbus/api/middleware/ratelimit"
	"gorm.io/gorm"
)

// InitUserRoutes registers the authentication endpoints under /v1/api/auth/.
func InitUserRoutes(r *gin.Engine, db *gorm.DB, s3Client *s3.Client) {
	r.GET("/register", user.ServeRegisterPage)

	// Throttle credential-guessing on login and password reset: 5 attempts per
	// 15 minutes, keyed by client IP + email so a single account can't be
	// hammered even from rotating IPs.
	authLimiter := ratelimit.New(5, 15*time.Minute)

	route := r.Group("v1/api/auth/")
	{
		route.POST("/users/register", func(c *gin.Context) {
			user.Register(c, db, s3Client)
		})
		route.POST("/users/login", authLimiter.Middleware(ratelimit.IPAndEmailKeys), func(c *gin.Context) {
			user.Login(c, db)
		})
		route.POST("/users/reset-password", authLimiter.Middleware(ratelimit.IPAndEmailKeys), func(c *gin.Context) {
			user.ResetPassword(c, db)
		})
	}
}
