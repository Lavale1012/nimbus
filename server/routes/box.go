package routes

import "github.com/gin-gonic/gin"

func InitBoxRoutes(r *gin.Engine) {
	route := r.Group("v1/api")
	{
		route.GET("/boxes")
		route.POST("/boxes")
		route.DELETE("/boxes/:id")
	}
}
