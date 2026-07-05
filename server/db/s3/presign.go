package s3

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// PresignPutObject generates a time-limited, pre-signed PUT URL for a specific
// S3 key. The CLI uses this URL to upload a file directly to S3 without
// sending the file data through the API server first — saving bandwidth and
// compute on the server side.
//
// contentLength binds the exact Content-Length into the signature: S3 rejects
// any upload whose Content-Length header doesn't match the signed value. This
// makes the server-side size cap real — a client can't be issued a URL for a
// small size and then push a much larger object through it. Pass 0 to skip
// binding the length (not recommended for user uploads).
func PresignPutObject(ctx context.Context, client *s3.Client, bucket, key, contentType string, contentLength int64, expiry time.Duration) (string, error) {
	presigner := s3.NewPresignClient(client)
	input := &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		ContentType: &contentType,
	}
	if contentLength > 0 {
		input.ContentLength = &contentLength
	}
	req, err := presigner.PresignPutObject(ctx, input, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// PresignGetObject generates a time-limited, pre-signed GET URL for a specific
// S3 key. The CLI uses this URL to download a file directly from S3 without
// proxying the bytes through the API server.
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
