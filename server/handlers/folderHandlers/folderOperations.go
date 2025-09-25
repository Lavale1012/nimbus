package folderhandlers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	config "github.com/nimbus/api/db/S3/config"
)

func CreateFolder(h config.AWS3ConfigFile, c *gin.Context) {
	// Implementation for creating a folder
	if h.Bucket == "" || h.S3 == nil {
		c.JSON(500, gin.H{"error": "S3 client or bucket not configured"})
		return
	}

	foldername := c.Query("foldername")
	if foldername == "" {
		c.JSON(400, gin.H{"error": "folder name is required"})
		return
	}

	// Sanitize the foldername and build a proper key
	base := filepath.Base(foldername)
	base = strings.ReplaceAll(base, " ", "_")
	if base == "" {
		base = "default_folder"
	}
	key := fmt.Sprintf(base + "/")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	_, err := h.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &h.Bucket,
		Key:    &key,
		Body:   strings.NewReader(""),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create folder"})
		return
	}

	c.JSON(200, gin.H{"message": "Folder created successfully", "folder": key})
}

func DownloadFolder(h config.AWS3ConfigFile, c *gin.Context) {}
func UploadFolder(h config.AWS3ConfigFile, c *gin.Context)   {}
