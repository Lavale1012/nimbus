package s3

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func PutObject(ctx context.Context, client *s3.Client, bucket string, key string, contentType string, file io.Reader) error {
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		Body:        file,
		ContentType: &contentType,
	})
	return err
}

func MakeUserFolder(ctx context.Context, client *s3.Client, bucket string, key string, contentType string) error {
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		ContentType: &contentType,
	})
	return err
}
