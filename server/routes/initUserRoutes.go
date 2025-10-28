package routes

import (
	"github.com/gin-gonic/gin"
	userhandlers "github.com/nimbus/api/handlers/userHandlers"
	"gorm.io/gorm"
)

func InitUserRoutes(r *gin.Engine, db *gorm.DB) {
	route := r.Group("v1/api/auth/")
	{
		route.POST("/users/register", func(c *gin.Context) {
			userhandlers.UserRegister(c, db)
		})
		route.POST("/users/login", func(c *gin.Context) {
			userhandlers.UserLogin(c, db)
		})
	}
}
