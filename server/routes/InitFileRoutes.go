package routes

import (
	"github.com/gin-gonic/gin"
	filehandlers "github.com/nimbus/api/handlers/fileHandlers"
)

func InitFileRoutes(r *gin.Engine, config filehandlers.AWS3Config) {
	// Initialize routes here

	route := r.Group("v1/api")
	{
		route.GET("/files", config.DownloadFile)
		route.POST("/files", config.UploadFile)
		route.DELETE("/files/:name", config.DeleteFile)
	}
}
