package s3

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func PresignPutObject(ctx context.Context, client *s3.Client, bucket, key, contentType string, expiry time.Duration) (string, error) {
	presigner := s3.NewPresignClient(client)
	req, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		ContentType: &contentType,
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func PresignGetObject(ctx context.Context, client *s3.Client, bucket, key string, expiry time.Duration) (string, error) {
	presigner := s3.NewPresignClient(client)
	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}
