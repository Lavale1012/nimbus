package folder

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
)

// TODO: Implement folder operations with Postgres integration
var CurrentFolder string
var FolderID uint = 34567890

func Create(h s3db.Config, c *gin.Context) {
	const slash string = "/"
	const MAX_FOLDER_NAME_LENGTH int8 = 25
	if h.Bucket == "" || h.Client == nil {
		c.JSON(500, gin.H{"error": "S3 client or bucket not configured"})
		return
	}

	foldername := c.Query("foldername")
	if foldername == "" {
		c.JSON(400, gin.H{"error": "folder name is required"})
		return
	}

	if int8(len(foldername)) > MAX_FOLDER_NAME_LENGTH {
		c.JSON(400, gin.H{"error": fmt.Sprintf("folder name must be at most %d characters", MAX_FOLDER_NAME_LENGTH)})
		return
	}

	// Sanitize the foldername and build a proper key
	base := filepath.Base(foldername)
	base = strings.ReplaceAll(base, " ", "_")

	key := fmt.Sprintf("%s%s", base, slash)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	_, err := h.Client.PutObject(ctx, &s3.PutObjectInput{
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

func Download(h s3db.Config, c *gin.Context) {}
func Upload(h s3db.Config, c *gin.Context)   {}
func Delete(h s3db.Config, c *gin.Context)   {}
func List(h s3db.Config, c *gin.Context)     {}
func Move(h s3db.Config, c *gin.Context)     {}
func Rename(h s3db.Config, c *gin.Context)   {}
func CurrentPath(h s3db.Config, c *gin.Context) (string, uint, error) {
	return CurrentFolder, FolderID, nil
}
