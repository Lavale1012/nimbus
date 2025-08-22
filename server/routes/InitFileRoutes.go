package routes

import (
	"github.com/gin-gonic/gin"
	filehandlers "github.com/nimbus/api/handlers/fileHandlers"
)

func InitFileRoutes(r *gin.Engine, uploader *filehandlers.Uploader) {
	// Initialize routes here

	route := r.Group("v1/api")
	{
		route.GET("/files")
		route.POST("/files", uploader.UploadFile)
		route.DELETE("/files/:id")
	}
}
