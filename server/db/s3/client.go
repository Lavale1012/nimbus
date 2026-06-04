// Package s3 provides helpers for connecting to S3 and performing common
// object operations (put, get, presign, bucket creation).
package s3

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Config bundles the S3 client and the bucket name together so handlers
// don't have to track them as separate arguments.
type Config struct {
	Client *s3.Client
	Bucket string
}

// Connect loads the default AWS credential chain (env vars, ~/.aws/credentials,
// EC2 instance role, etc.) for the given region and returns a ready-to-use S3
// client. When running locally with LocalStack the AWS SDK reads S3_ENDPOINT
// from the environment automatically via the default config loader.
func Connect(ctx context.Context, region string) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = false }), nil
}
