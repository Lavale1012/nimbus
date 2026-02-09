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

// ValidateBoxOwnership validates box exists and belongs to user
func ValidateBoxOwnership(db *gorm.DB, boxName string, userID uint) (*models.Box, error) {
	var box models.Box
	if err := db.Where("name = ? AND user_id = ?", boxName, userID).First(&box).Error; err != nil {
		return nil, fmt.Errorf("box not found or access denied")
	}
	return &box, nil
}

// GenerateS3Key generates a unique S3 key
func GenerateS3Key(filePath, filename, boxName string, user *models.User) (string, error) {
	fullFilePathPrefix := fmt.Sprintf("users/nim-user-%d/boxes/%s", user.ID, boxName)
	base := filepath.Base(filename)
	base = strings.ReplaceAll(base, " ", "_")
	if base == "" {
		return "", fmt.Errorf("invalid filename")
	}
	timestamp := time.Now().Unix()
	fullPath := fmt.Sprintf("%s%s/%s_%d", fullFilePathPrefix, filePath, base, timestamp)
	return fullPath, nil
}

// AssociateFileWithFolder associates file with folder if provided
func AssociateFileWithFolder(db *gorm.DB, c *gin.Context, fileModel *models.File, boxID uint) {
	strFolderID := c.PostForm("folder_id")
	if strFolderID == "" {
		return
	}

	folderID, err := strconv.Atoi(strFolderID)
	if err != nil || folderID <= 0 {
		return
	}

	var folder models.Folder
	if err := db.First(&folder, uint(folderID)).Error; err != nil {
		return
	}

	// Only associate if folder belongs to the same box
	// if folder.BoxID == boxID {
	// 	db.Model(fileModel).Association("Folders").Append(&folder)
	// }
	if folder.BoxID == boxID {
		fid := uint(folderID)
		fileModel.FolderID = &fid
		db.Save(fileModel)
	}
}

// GetBoxID returns the box ID for a given box name and user
func GetBoxID(db *gorm.DB, boxName string, userID uint) uint {
	var box models.Box
	if err := db.Where("name = ? AND user_id = ?", boxName, userID).First(&box).Error; err != nil {
		return 0
	}
	return box.ID
}

// GetParentFolderID resolves a path to a folder ID
// Returns nil if path is empty (root level), or the folder ID if path exists
func GetParentFolderID(db *gorm.DB, userID uint, boxName string, path string) *uint {
	if path == "" || path == "/" {
		return nil
	}

	// Get box ID first
	boxID := GetBoxID(db, boxName, userID)
	if boxID == 0 {
		return nil
	}

	// Clean and split the path
	path = strings.Trim(path, "/")
	segments := strings.Split(path, "/")

	// Traverse the folder hierarchy
	var currentParentID *uint = nil

	for _, segment := range segments {
		var folder models.Folder
		query := db.Where("name = ? AND user_id = ? AND box_id = ?", segment, userID, boxID)

		if currentParentID == nil {
			query = query.Where("parent_id IS NULL")
		} else {
			query = query.Where("parent_id = ?", *currentParentID)
		}

		if err := query.First(&folder).Error; err != nil {
			// Folder not found in path
			return nil
		}

		currentParentID = &folder.ID
	}

	return currentParentID
}

// // reverseString reverses a string, handling Unicode correctly.
// func ReverseString(s string) string {
// 	runes := []rune(s) // Convert to runes for Unicode safety
// 	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
// 		runes[i], runes[j] = runes[j], runes[i] // Swap runes
// 	}
// 	return string(runes)
// }

// func TrimReverseUntil(s string, char rune) string {
// 	// 1. Reverse the original string
// 	reversedS := ReverseString(s)

// 	// 2. Trim characters from the right (original left) until 'char' is found
// 	// We need to find the index of 'char' in the reversed string
// 	// and then slice up to that point.
// 	// Using strings.IndexRune is efficient.
// 	index := strings.IndexRune(reversedS, char)

// 	// If the character is found, take the substring up to that character
// 	if index != -1 {
// 		reversedS = reversedS[:index] // Keep everything before the char
// 	} else {
// 		// If char not found, maybe return empty or original reversed (depends on requirement)
// 		// Here, we'll return empty as we couldn't find the stop point.
// 		return ""
// 	}

// 	// 3. Reverse the result back to original orientation
// 	return ReverseString(reversedS)
// }
