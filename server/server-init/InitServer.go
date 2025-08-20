package server

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/nimbus/api/routes"
)

func InitServer() error {
	r := gin.Default()
	r.Use(gin.Recovery())
	r.Use(cors.Default())

	// Initialize routes here
	routes.InitFileRoutes(r)
	routes.InitBoxRoutes(r)
	routes.InitSectionRoutes(r)
	r.Run("localhost:8080")
	return nil
}
