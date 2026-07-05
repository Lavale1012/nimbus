// Package routes wires HTTP verbs + paths to handler functions.
// Each Init* function is called once at startup from server-init/server.go.
package routes

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/nimbus/api/handlers/user"
	"github.com/nimbus/api/middleware/ratelimit"
	"gorm.io/gorm"
)

// InitUserRoutes registers the authentication endpoints under /v1/api/auth/.
// authLimiter throttles credential-guessing on login and password reset (keyed
// by client IP + email); it is built in the bootstrap so it can be Redis-backed
// (shared across instances) or in-memory depending on configuration.
func InitUserRoutes(r *gin.Engine, db *gorm.DB, s3Client *s3.Client, authLimiter *ratelimit.Limiter) {
	r.GET("/register", user.ServeRegisterPage)

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
