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
// func ParseBoxID(c *gin.Context) (uint, error) {
// 	strBoxID := c.Query("box_id")
// 	if strBoxID == "" {
// 		return 0, fmt.Errorf("box_id is required")
// 	}

// 	intBoxID, err := strconv.Atoi(strBoxID)
// 	if err != nil || intBoxID <= 0 {
// 		return 0, fmt.Errorf("invalid box_id")
// 	}

// 	return uint(intBoxID), nil
// }

// Helper function: Validate box exists and belongs to user
func ValidateBoxOwnership(db *gorm.DB, boxName string, userID uint) (*models.BoxModel, error) {
	var box models.BoxModel
	if err := db.Where("name = ? AND user_id = ?", boxName, userID).First(&box).Error; err != nil {
		return nil, fmt.Errorf("box not found or access denied")
	}
	return &box, nil
}

// Helper function: Generate unique S3 key
func GenerateS3Key(filePath, filename, boxName string, user *models.UserModel) (string, error) {
	fullFilePathPrefix := fmt.Sprintf("users/nim-user-%d/boxes/%s/", user.ID, boxName)
	base := filepath.Base(filename)
	base = strings.ReplaceAll(base, " ", "_")
	if base == "" {
		return "", fmt.Errorf("invalid filename")
	}
	timestamp := time.Now().Unix()
	fullPath := fmt.Sprintf("%s%s/%s_%d", fullFilePathPrefix, filePath, base, timestamp)
	return fullPath, nil
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
