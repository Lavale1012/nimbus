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
			file.Download(config, c, db)
		})
		route.POST("/files", func(c *gin.Context) {
			file.Upload(config, db, c)
		})
		route.DELETE("/files/:name", func(c *gin.Context) {
			file.Delete(config, db, c)
		})
	}
}
