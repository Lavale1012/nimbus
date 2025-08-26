package filehandlers

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

type Downloader struct {
	S3     *s3.Client
	Bucket string
}

func (d Downloader) DownloadFile(c *gin.Context) {
	if d.S3 == nil || d.Bucket == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "downloader not configured: missing S3 client or bucket"})
		return
	}
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file key is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Proceed with downloading the file from S3
	obj, err := d.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &d.Bucket,
		Key:    &key,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get object: %v", err)})
		return
	}
	defer obj.Body.Close()

	ct := "application/octet-stream"

	if obj.ContentType != nil && *obj.ContentType != "" {
		ct = *obj.ContentType
	}

	filename := filepath.Base(key)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.DataFromReader(http.StatusOK, *obj.ContentLength, ct, obj.Body, nil)
}
