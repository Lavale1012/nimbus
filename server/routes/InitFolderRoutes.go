package routes

import "github.com/gin-gonic/gin"

func InitFolderRoutes(r *gin.Engine) {
	route := r.Group("v1/api")
	{
		route.GET("/folders")
		route.POST("/folders")
		route.DELETE("/folders/:id")
	}
}
