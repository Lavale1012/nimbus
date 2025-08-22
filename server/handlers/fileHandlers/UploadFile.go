package filehandlers

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	s3Operations "github.com/nimbus/api/db/S3/operations"
)

type Uploader struct {
	// Define any fields you need for the uploader
	S3     *s3.Client
	Bucket string
}

func (h Uploader) UploadFile(c *gin.Context) {
	// Guard against nil client or empty bucket to avoid panics
	if h.S3 == nil || h.Bucket == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "uploader not configured: missing S3 client or bucket"})
		return
	}

	// Handle file upload logic here
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "File is required"})
		return
	}
	defer file.Close()

	// Sanitize the filename and build a namespaced, collision-proof key
	base := filepath.Base(header.Filename)
	base = strings.ReplaceAll(base, " ", "_")
	if base == "" {
		base = "upload.bin"
	}
	key := fmt.Sprintf(base)

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Use request context with timeout so slow S3 calls don't hang the handler
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	err = s3Operations.PutObject(ctx, h.S3, h.Bucket, key, contentType, file)
	if err != nil {
		// Return the actual error message to help debug (consider hiding in prod)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "file uploaded",
		"bucket":  h.Bucket,
		"key":     key,
		"url":     fmt.Sprintf("s3://%s/%s", h.Bucket, key),
	})
}
