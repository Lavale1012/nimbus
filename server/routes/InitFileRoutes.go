package routes

import (
	"github.com/gin-gonic/gin"
	config "github.com/nimbus/api/db/S3/config"
	filehandlers "github.com/nimbus/api/handlers/fileHandlers"
)

func InitFileRoutes(r *gin.Engine, config config.AWS3ConfigFile) {
	// Initialize routes here

	route := r.Group("v1/api")
	{
		route.GET("/files", func(c *gin.Context) {
			filehandlers.DownloadFile(config, c)
		})
		route.POST("/files", func(c *gin.Context) {
			filehandlers.UploadFile(config, c)
		})
		route.DELETE("/files/:name", func(c *gin.Context) {
			filehandlers.DeleteFile(config, c)
		})
	}
}
