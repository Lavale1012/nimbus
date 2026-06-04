package s3

import (
	"context"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// GetObject downloads the S3 object at key and writes it to a local file at
// outPath. The file is created (or truncated) before streaming begins.
func GetObject(ctx context.Context, client *s3.Client, bucket string, key string, outPath string) error {
	obj, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return err
	}
	defer obj.Body.Close()

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, obj.Body)
	return err
}
