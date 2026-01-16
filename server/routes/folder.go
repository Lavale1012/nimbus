package routes

import (
	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
	"github.com/nimbus/api/handlers/folder"
)

func InitFolderRoutes(r *gin.Engine, config s3db.Config) {
	route := r.Group("v1/api")
	{
		route.GET("/folders", func(c *gin.Context) {
			folder.Download(config, c)
		})
		route.POST("/folders", func(c *gin.Context) {
			folder.Create(config, c)
		})
		route.POST("/folders/upload", func(c *gin.Context) {
			folder.Upload(config, c)
		})
		route.DELETE("/folders/:id", func(c *gin.Context) {
			// Implement delete folder handler here
		})
	}
}
