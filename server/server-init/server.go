package server

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/nimbus/api/db/postgres"
	s3db "github.com/nimbus/api/db/s3"
	"github.com/nimbus/api/routes"
	"github.com/nimbus/api/utils"
	"gorm.io/gorm"
)

var S3 *s3.Client
var DB *gorm.DB

func InitServer() error {
	bucket, err := utils.GetEnv("S3_BUCKET")
	if err != nil {
		return err
	}
	r := gin.Default()
	r.Use(gin.Recovery())
	r.Use(cors.Default())

	ctx := context.Background()

	region, err := utils.GetEnv("AWS_REGION")
	if err != nil {
		return err
	}

	s3Client, err := s3db.Connect(ctx, region)
	if err != nil {
		return err
	}

	S3 = s3Client
	if S3 == nil {
		return fmt.Errorf("failed to connect to S3")
	}

	config := s3db.Config{
		Client: S3,
		Bucket: bucket,
	}

	DB, err = postgres.Connect()
	if err != nil {
		return err
	}
	if DB == nil {
		return fmt.Errorf("failed to connect to PostgreSQL")
	}
	routes.InitFileRoutes(r, config, DB)
	routes.InitBoxRoutes(r)
	routes.InitFolderRoutes(r, config, DB)
	routes.InitUserRoutes(r, DB, S3)

	r.Run("localhost:8080")
	return nil
}
