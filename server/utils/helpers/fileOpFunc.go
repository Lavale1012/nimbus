package helpers

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nimbus/api/models"
	"gorm.io/gorm"
)

// Helper function: Parse box_id from query parameters
func ParseBoxID(c *gin.Context) (uint, error) {
	strBoxID := c.Query("box_id")
	if strBoxID == "" {
		return 0, fmt.Errorf("box_id is required")
	}

	intBoxID, err := strconv.Atoi(strBoxID)
	if err != nil || intBoxID <= 0 {
		return 0, fmt.Errorf("invalid box_id")
	}

	return uint(intBoxID), nil
}

// Helper function: Validate box exists and belongs to user
func ValidateBoxOwnership(db *gorm.DB, boxID, userID uint) (*models.BoxModel, error) {
	var box models.BoxModel
	if err := db.Where("box_id = ? AND user_id = ?", boxID, userID).First(&box).Error; err != nil {
		return nil, fmt.Errorf("box not found or access denied")
	}
	return &box, nil
}

// Helper function: Generate unique S3 key
func GenerateS3Key(bucketPrefix, filename string) string {
	base := filepath.Base(filename)
	base = strings.ReplaceAll(base, " ", "_")
	if base == "" {
		base = "upload.bin"
	}

	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s%s_%d", bucketPrefix, base, timestamp)
}

// Helper function: Associate file with folder if provided
func AssociateFileWithFolder(db *gorm.DB, c *gin.Context, fileModel *models.FileModel, boxID uint) {
	strFolderID := c.PostForm("folder_id")
	if strFolderID == "" {
		return
	}

	folderID, err := strconv.Atoi(strFolderID)
	if err != nil || folderID <= 0 {
		return
	}

	var folder models.FolderModel
	if err := db.First(&folder, uint(folderID)).Error; err != nil {
		return
	}

	// Only associate if folder belongs to the same box
	if folder.BoxID == boxID {
		db.Model(fileModel).Association("Folders").Append(&folder)
	}
}
