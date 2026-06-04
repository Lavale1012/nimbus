// Package routes wires HTTP verbs + paths to handler functions.
// Each Init* function is called once at startup from server-init/server.go.
package routes

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/nimbus/api/handlers/user"
	"gorm.io/gorm"
)

// InitUserRoutes registers the authentication endpoints under /v1/api/auth/.
func InitUserRoutes(r *gin.Engine, db *gorm.DB, s3Client *s3.Client) {
	route := r.Group("v1/api/auth/")
	{
		route.POST("/users/register", func(c *gin.Context) {
			user.Register(c, db, s3Client)
		})
		route.POST("/users/login", func(c *gin.Context) {
			user.Login(c, db)
		})
	}
}
