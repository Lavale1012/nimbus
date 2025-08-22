package s3Operations

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func MakeBucket(ctx context.Context, s3c *s3.Client, bucket string, region string) error {
	input := &s3.CreateBucketInput{
		Bucket: &bucket,
	}

	// Only add CreateBucketConfiguration for regions other than us-east-1
	// LocalStack requires this for proper region handling
	if region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		}
	}

	_, err := s3c.CreateBucket(ctx, input)
	return err
}
