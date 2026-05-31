package routes

import (
	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
	"github.com/nimbus/api/handlers/file"
	"gorm.io/gorm"
)

func InitFileRoutes(r *gin.Engine, config s3db.Config, db *gorm.DB) {
	route := r.Group("v1/api")
	{
		route.GET("/files", func(c *gin.Context) {
			file.List(config, db, c)
		})
		route.GET("/files/presign-download", func(c *gin.Context) {
			file.PresignDownload(config, c, db)
		})
		route.POST("/files/presign-upload", func(c *gin.Context) {
			file.PresignUpload(config, db, c)
		})
		route.DELETE("/files/:name", func(c *gin.Context) {
			file.Delete(config, db, c)
		})
		route.PATCH("/files/rename", func(c *gin.Context) {
			file.Rename(config, db, c)
		})
		route.PATCH("/files/move", func(c *gin.Context) {
			file.Move(config, db, c)
		})
	}
}
