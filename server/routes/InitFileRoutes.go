package routes

import "github.com/gin-gonic/gin"

func InitFileRoutes(r *gin.Engine) {
	// Initialize routes here

	route := r.Group("v1/api")
	{
		route.GET("/files")
		route.POST("/files")
		route.DELETE("/files/:id")
	}
}
