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
	const slash string = "/"
	const MAX_FOLDER_NAME_LENGTH int = 255
	if h.Bucket == "" || h.S3 == nil {
		c.JSON(500, gin.H{"error": "S3 client or bucket not configured"})
		return
	}

	foldername := c.Query("foldername")
	if foldername == "" {
		c.JSON(400, gin.H{"error": "folder name is required"})
		return
	}

	if len(foldername) > MAX_FOLDER_NAME_LENGTH {
		c.JSON(400, gin.H{"error": fmt.Sprintf("folder name must be at most %d characters", MAX_FOLDER_NAME_LENGTH)})
		return
	}

	// Sanitize the foldername and build a proper key
	base := filepath.Base(foldername)
	base = strings.ReplaceAll(base, " ", "_")

	key := fmt.Sprintf("%s%s", base, slash)

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
func DeleteFolder(h config.AWS3ConfigFile, c *gin.Context)   {}
func ListFolders(h config.AWS3ConfigFile, c *gin.Context)    {}
func MoveFolder(h config.AWS3ConfigFile, c *gin.Context)     {}
func RenameFolder(h config.AWS3ConfigFile, c *gin.Context)   {}
