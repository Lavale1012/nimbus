package s3

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// PutObject uploads the contents of file directly to the given key in the
// bucket. This is used for server-side uploads where the data flows through
// the API rather than via a presigned URL.
func PutObject(ctx context.Context, client *s3.Client, bucket string, key string, contentType string, file io.Reader) error {
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		Body:        file,
		ContentType: &contentType,
	})
	return err
}

// MakeUserFolder creates a zero-byte "folder" object in S3 using a key that
// ends with a trailing slash (e.g. "users/nim-user-1/boxes/Home-Box/").
// S3 is a flat key-value store — folders don't really exist, but this
// placeholder lets the S3 console and other tools display a folder structure.
func MakeUserFolder(ctx context.Context, client *s3.Client, bucket string, key string, contentType string) error {
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		ContentType: &contentType,
	})
	return err
}
