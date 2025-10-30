package routes

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	userhandlers "github.com/nimbus/api/handlers/userHandlers"
	"gorm.io/gorm"
)

func InitUserRoutes(r *gin.Engine, db *gorm.DB, s3Client *s3.Client) {
	route := r.Group("v1/api/auth/")
	{
		route.POST("/users/register", func(c *gin.Context) {
			userhandlers.UserRegister(c, db, s3Client)
		})
		route.POST("/users/login", func(c *gin.Context) {
			userhandlers.UserLogin(c, db)
		})
	}
}
