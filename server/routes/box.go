package routes

import (
	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
	"github.com/nimbus/api/handlers/box"
	"gorm.io/gorm"
)

func InitBoxRoutes(r *gin.Engine, config s3db.Config, db *gorm.DB) {
	route := r.Group("v1/api")
	{
		route.GET("/boxes", func(c *gin.Context) {
			box.ListBoxes(config, c, db)
		})
		route.POST("/boxes", func(c *gin.Context) {
			box.CreateBox(config, c, db)
		})
		route.DELETE("/boxes", func(c *gin.Context) {
			box.DeleteBox(config, c, db)
		})
	}
}
