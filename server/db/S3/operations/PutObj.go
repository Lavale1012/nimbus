package s3Operations

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func PutObject(ctx context.Context, s3c *s3.Client, Bucket string, key string, contentType string, file io.Reader) error {

	_, err := s3c.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &Bucket,
		Key:         &key,
		Body:        file,
		ContentType: &contentType,
	})
	return err
}
