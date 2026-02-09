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

// ListResponse represents the contents of a folder
type ListResponse struct {
	FolderID   *uint         `json:"folder_id"`
	FolderName string        `json:"folder_name"`
	Path       string        `json:"path"`
	Files      []FileEntry   `json:"files"`
	Folders    []FolderEntry `json:"folders"`
}

type FileEntry struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	S3Key     string `json:"s3_key"`
	CreatedAt string `json:"created_at"`
}

type FolderEntry struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

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
	} else {
		err = db.Create(&models.Folder{
			Name:     sanitizedName,
			UserID:   user.ID,
			BoxID:    helpers.GetBoxID(db, boxName, user.ID),
			ParentID: helpers.GetParentFolderID(db, user.ID, boxName, Path),
		}).Error
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to create folder in database"})
			return
		}
	}

	c.JSON(200, gin.H{"message": "Folder created successfully", "folder": key})
}

func Download(h s3db.Config, c *gin.Context) {}
func Upload(h s3db.Config, c *gin.Context)   {}
func Delete(h s3db.Config, c *gin.Context)   {}

func List(h s3db.Config, c *gin.Context, db *gorm.DB) {
	// Authenticate user
	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	// Get box name from query params
	boxName := c.Query("box_name")
	if boxName == "" {
		c.JSON(400, gin.H{"error": "box_name is required"})
		return
	}

	// Validate box ownership
	box, err := helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		c.JSON(403, gin.H{"error": "box not found or access denied"})
		return
	}

	// Get optional path from query params (empty means root of box)
	// Path format: "folder1/folder2/folder3" or empty for root
	pathParam := c.Query("path")

	var folderID *uint
	var folderName string
	var path string

	if pathParam != "" && pathParam != "/" {
		// Clean the path and split into segments
		pathParam = strings.Trim(pathParam, "/")
		segments := strings.Split(pathParam, "/")

		// Traverse the folder hierarchy to find the target folder
		var currentParentID *uint = nil

		for i, segment := range segments {
			var folder models.Folder
			query := db.Where("name = ? AND user_id = ? AND box_id = ?", segment, user.ID, box.ID)

			if currentParentID == nil {
				query = query.Where("parent_id IS NULL")
			} else {
				query = query.Where("parent_id = ?", *currentParentID)
			}

			if err := query.First(&folder).Error; err != nil {
				c.JSON(404, gin.H{"error": fmt.Sprintf("folder not found: %s", segment)})
				return
			}

			// If this is the last segment, this is our target folder
			if i == len(segments)-1 {
				folderID = &folder.ID
				folderName = folder.Name
				path = "/" + pathParam
			} else {
				currentParentID = &folder.ID
			}
		}
	} else {
		folderName = boxName
		path = "/"
	}

	// Query files in this folder
	var files []models.File
	if folderID == nil {
		// Root level: files with no folder
		db.Where("user_id = ? AND box_id = ? AND folder_id IS NULL", user.ID, box.ID).Find(&files)
	} else {
		db.Where("user_id = ? AND box_id = ? AND folder_id = ?", user.ID, box.ID, *folderID).Find(&files)
	}

	// Query subfolders
	var subfolders []models.Folder
	if folderID == nil {
		// Root level: folders with no parent
		db.Where("user_id = ? AND box_id = ? AND parent_id IS NULL", user.ID, box.ID).Find(&subfolders)
	} else {
		db.Where("user_id = ? AND box_id = ? AND parent_id = ?", user.ID, box.ID, *folderID).Find(&subfolders)
	}

	// Build response
	fileEntries := make([]FileEntry, len(files))
	for i, f := range files {
		fileEntries[i] = FileEntry{
			ID:        f.ID,
			Name:      f.Name,
			Size:      f.Size,
			S3Key:     f.S3Key,
			CreatedAt: f.CreatedAt.Format(time.RFC3339),
		}
	}

	folderEntries := make([]FolderEntry, len(subfolders))
	for i, f := range subfolders {
		folderEntries[i] = FolderEntry{
			ID:        f.ID,
			Name:      f.Name,
			CreatedAt: f.CreatedAt.Format(time.RFC3339),
		}
	}

	response := ListResponse{
		FolderID:   folderID,
		FolderName: folderName,
		Path:       path,
		Files:      fileEntries,
		Folders:    folderEntries,
	}

	c.JSON(200, response)
}
func Move(h s3db.Config, c *gin.Context)   {}
func Rename(h s3db.Config, c *gin.Context) {}
