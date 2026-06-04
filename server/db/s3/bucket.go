package s3

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// MakeBucket creates an S3 bucket in the given region.
// us-east-1 is a special case in AWS: it does NOT accept a
// LocationConstraint in the create request — passing one returns an error.
// Every other region requires the constraint so S3 knows where to place the bucket.
func MakeBucket(ctx context.Context, client *s3.Client, bucket string, region string) error {
	input := &s3.CreateBucketInput{
		Bucket: &bucket,
	}

	if region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		}
	}

	_, err := client.CreateBucket(ctx, input)
	return err
}
