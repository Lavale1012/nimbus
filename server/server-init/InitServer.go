package server

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	aws "github.com/nimbus/api/db/S3/config"
	filehandlers "github.com/nimbus/api/handlers/fileHandlers"
	"github.com/nimbus/api/routes"
	"github.com/nimbus/api/utils"
)

var S3 *s3.Client

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

	config := filehandlers.AWS3Config{
		S3:     S3,
		Bucket: bucket,
	}
	routes.InitFileRoutes(r, config)
	routes.InitBoxRoutes(r)
	routes.InitFolderRoutes(r)

	r.Run("localhost:8080")
	return nil
}
