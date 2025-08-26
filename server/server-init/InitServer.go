package server

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	aws "github.com/nimbus/api/db/S3/config"
	filehandlers "github.com/nimbus/api/handlers/fileHandlers"
	"github.com/nimbus/api/routes"
)

var S3 *s3.Client

func InitServer() error {

	r := gin.Default()
	r.Use(gin.Recovery())
	r.Use(cors.Default())

	ctx := context.Background()

	region := "us-east-2" //TODO: function will get this data

	s3, err := aws.ConnectToS3(ctx, region)
	if err != nil {
		return err
	}

	S3 = s3

	uploader := &filehandlers.Uploader{
		S3:     S3,
		Bucket: "nimbus-cli-storage", // TODO: a function later will get the bucket name
	}
	downloader := &filehandlers.Downloader{
		S3:     S3,
		Bucket: "nimbus-cli-storage", // TODO: a function later will get the bucket name
	}
	routes.InitFileRoutes(r, uploader, downloader)
	routes.InitBoxRoutes(r)
	routes.InitSectionRoutes(r)

	r.Run("localhost:8080")
	return nil
}
