package routes

import (
	"github.com/gin-gonic/gin"
	config "github.com/nimbus/api/db/S3/config"
	folderhandlers "github.com/nimbus/api/handlers/folderHandlers"
)

func InitFolderRoutes(r *gin.Engine, config config.AWS3ConfigFile) {
	route := r.Group("v1/api")
	{
		route.GET("/folders", func(c *gin.Context) {
			folderhandlers.DownloadFolder(config, c)
		})
		route.POST("/folders", func(c *gin.Context) {
			folderhandlers.CreateFolder(config, c)
		})
		route.POST("/folders/upload", func(c *gin.Context) {
			folderhandlers.UploadFolder(config, c)
		})
		route.DELETE("/folders/:id", func(c *gin.Context) {
			// Implement delete folder handler here
		})
	}
}
