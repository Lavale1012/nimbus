package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type AWS3ConfigFile struct {
	S3     *s3.Client
	Bucket string
}

func ConnectToS3(ctx context.Context, region string) (*s3.Client, error) {
	// Implementation for connecting to S3
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = false }), nil
}
