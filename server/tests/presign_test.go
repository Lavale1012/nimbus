package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3db "github.com/nimbus/api/db/s3"
	"github.com/stretchr/testify/assert"
)

// newTestS3Client builds an S3 client with static dummy credentials. Presigning
// is a purely local signing operation, so no network access or real AWS account
// is needed — this exercises PresignPutObject offline.
func newTestS3Client() *s3.Client {
	return s3.New(s3.Options{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKIDTEST", "SECRETTEST", ""),
	})
}

// A presigned PUT that binds a content length signs the Content-Length header,
// which surfaces in the query string as a signed header. This is what makes S3
// reject an upload whose actual Content-Length differs from the signed size.
func TestPresignPutObject_BindsContentLength(t *testing.T) {
	client := newTestS3Client()

	url, err := s3db.PresignPutObject(
		context.Background(), client,
		"test-bucket", "users/1/boxes/Home/file.bin",
		"application/octet-stream",
		1234,          // content length
		5*time.Minute, // expiry
	)
	assert.NoError(t, err)
	assert.NotEmpty(t, url)

	// content-length must be part of the signed headers.
	assert.Contains(t, strings.ToLower(url), "content-length",
		"presigned URL should sign the content-length header when a size is bound")
}

func TestPresignPutObject_ZeroLengthSkipsBinding(t *testing.T) {
	client := newTestS3Client()

	url, err := s3db.PresignPutObject(
		context.Background(), client,
		"test-bucket", "users/1/boxes/Home/file.bin",
		"application/octet-stream",
		0, // skip binding
		5*time.Minute,
	)
	assert.NoError(t, err)
	assert.NotEmpty(t, url)

	// With no bound length, content-length is not among the signed headers.
	assert.NotContains(t, strings.ToLower(url), "content-length",
		"presigned URL should not sign content-length when size is 0")
}

// Guard against the exported signature drifting: content type must still be
// accepted and produce a valid URL.
func TestPresignPutObject_ProducesURL(t *testing.T) {
	client := newTestS3Client()
	url, err := s3db.PresignPutObject(
		context.Background(), client,
		"b", "k", "text/plain", 10, time.Minute,
	)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(url, "https://"), "expected an https presigned URL, got %q", url)
}
