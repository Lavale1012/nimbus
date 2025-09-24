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
	config "github.com/nimbus/api/db/S3/config"
	s3Operations "github.com/nimbus/api/db/S3/operations"
)

func DownloadFile(d config.AWS3ConfigFile, c *gin.Context) {
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
	// Map not found to 404 if AWS returned NoSuchKey
	// (string contains check keeps it simple without importing API error types)
	if obj == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "object not found"})
		return
	}
	defer obj.Body.Close()

	ct := "application/octet-stream"

	if obj.ContentType != nil && *obj.ContentType != "" {
		ct = *obj.ContentType
	}

	filename := filepath.Base(key)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Type", ct)
	c.DataFromReader(http.StatusOK, *obj.ContentLength, ct, obj.Body, nil)
}

func UploadFile(h config.AWS3ConfigFile, c *gin.Context) {
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

func DeleteFile(d config.AWS3ConfigFile, c *gin.Context) {
	// Implementation for deleting a file from S3
	keyName := c.Param("name")
	if keyName == "" {
		c.JSON(400, gin.H{"error": "file name is required"})
		return
	}

	if d.S3 == nil || d.Bucket == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "downloader not configured: missing S3 client or bucket"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	_, err := d.S3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &d.Bucket,
		Key:    &keyName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to delete object: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file deleted"})

}
