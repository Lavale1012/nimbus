package server

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	postgres "github.com/nimbus/api/db/Postgres/config"
	aws "github.com/nimbus/api/db/S3/config"
	config "github.com/nimbus/api/db/S3/config"
	"github.com/nimbus/api/routes"
	"github.com/nimbus/api/utils"
	"gorm.io/gorm"
)

var S3 *s3.Client
var DB *gorm.DB

func InitServer() error {
	bucket, err := utils.GetEnv("S3_BUCKET") // TODO: A function will get this data
	if err != nil {
		return err
	}
	r := gin.Default()
	r.Use(gin.Recovery())
	r.Use(cors.Default())

	ctx := context.Background()

	region, err := utils.GetEnv("AWS_REGION") // TODO: A function will get this data
	if err != nil {
		return err
	}

	s3, err := aws.ConnectToS3(ctx, region)
	if err != nil {
		return err
	}

	S3 = s3
	if S3 == nil {
		return fmt.Errorf("failed to connect to S3")
	}

	config := config.AWS3ConfigFile{
		S3:     S3,
		Bucket: bucket,
	}

	DB, err = postgres.ConnectPostgres()
	if err != nil {
		return err
	}
	if DB == nil {
		return fmt.Errorf("failed to connect to PostgreSQL")
	}
	routes.InitFileRoutes(r, config, DB)
	routes.InitBoxRoutes(r)
	routes.InitFolderRoutes(r, config)

	r.Run("localhost:8080")
	return nil
}
