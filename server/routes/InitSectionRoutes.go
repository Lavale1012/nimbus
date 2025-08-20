package routes

import "github.com/gin-gonic/gin"

func InitSectionRoutes(r *gin.Engine) {
	route := r.Group("v1/api")
	{
		route.GET("/sections")
		route.POST("/sections")
		route.DELETE("/sections/:id")
	}
}
