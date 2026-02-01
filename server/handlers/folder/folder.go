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
	"github.com/nimbus/api/middleware/jwt"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils/helpers"
	"gorm.io/gorm"
)

// TODO: Implement folder operations with Postgres integration

func Create(h s3db.Config, c *gin.Context, db *gorm.DB) {
	var err error
	var user *models.User
	user, err = jwt.AuthenticateUser(c, db)
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	const MAX_FOLDER_NAME_LENGTH int = 25

	if h.Bucket == "" || h.Client == nil {
		c.JSON(500, gin.H{"error": "S3 client or bucket not configured"})
		return
	}

	boxName := c.Query("box_name")
	if boxName == "" {
		c.JSON(400, gin.H{"error": "box name is required"})
		return
	}

	foldername := c.Query("folder_name")
	if foldername == "" {
		c.JSON(400, gin.H{"error": "folder name is required"})
		return
	}

	if len(foldername) > MAX_FOLDER_NAME_LENGTH {
		c.JSON(400, gin.H{"error": fmt.Sprintf("folder name must be at most %d characters", MAX_FOLDER_NAME_LENGTH)})
		return
	}

	_, err = helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		c.JSON(403, gin.H{"error": "box not found or access denied"})
		return
	}

	// TODO: implement path validation

	// Sanitize the folder name
	sanitizedName := filepath.Base(foldername)
	sanitizedName = strings.ReplaceAll(sanitizedName, " ", "_")

	Path := c.Query("path")

	// Build the S3 key with trailing slash to represent a folder
	var key string

	if Path == "" {
		key = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/", user.ID, boxName, sanitizedName)
	} else {
		key = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/%s/", user.ID, boxName, Path, sanitizedName)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	_, err = h.Client.PutObject(ctx, &s3.PutObjectInput{
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

// func CurrentPath(h s3db.Config, c *gin.Context) (string, uint, error) {
// 	return CurrentFolder, FolderID, nil
// }
