// Package helpers provides reusable business-logic utilities shared across
// multiple handlers — box ownership checks, S3 key generation, and folder
// path resolution.
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

// ValidateBoxOwnership looks up a box by name and owner. It returns the Box
// record so callers can use its ID without a second query, or an error if the
// box doesn't exist or belongs to a different user.
func ValidateBoxOwnership(db *gorm.DB, boxName string, userID uint) (*models.Box, error) {
	var box models.Box
	if err := db.Where("name = ? AND user_id = ?", boxName, userID).First(&box).Error; err != nil {
		return nil, fmt.Errorf("box not found or access denied")
	}
	return &box, nil
}

// GenerateS3Key builds the full S3 object key for a file being uploaded.
// The format is:
//
//	users/nim-user-<userID>/boxes/<boxName><filePath>/<filename>_<unix_timestamp>
//
// The timestamp suffix prevents collisions when the same filename is uploaded
// to the same path multiple times.
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

// AssociateFileWithFolder reads an optional "folder_id" field from the
// multipart form body and, if present, links the file to that folder in the
// database. It silently returns if the field is missing or the folder doesn't
// belong to the same box — this keeps the upload flow working even without a
// folder target.
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

	// Only associate if the folder lives inside the same box as the file.
	if folder.BoxID == boxID {
		fid := uint(folderID)
		fileModel.FolderID = &fid
		db.Save(fileModel)
	}
}

// GetBoxID is a convenience wrapper that returns the database primary key (ID)
// of a box given its name and owner. Returns 0 if the box is not found.
func GetBoxID(db *gorm.DB, boxName string, userID uint) uint {
	var box models.Box
	if err := db.Where("name = ? AND user_id = ?", boxName, userID).First(&box).Error; err != nil {
		return 0
	}
	return box.ID
}

// GetParentFolderID resolves a slash-separated path string to the database ID
// of the deepest folder in that path. It walks the folder tree one segment at
// a time, always scoping to the correct box and parent.
//
// Returns nil when path is empty (meaning "the root of the box"), or nil if
// any segment in the path doesn't exist.
//
// Example: path "documents/projects" walks:
//  1. Find "documents" at root → get its ID
//  2. Find "projects" whose parent is "documents" → return its ID
func GetParentFolderID(db *gorm.DB, userID uint, boxName string, path string) *uint {
	if path == "" || path == "/" {
		return nil
	}

	boxID := GetBoxID(db, boxName, userID)
	if boxID == 0 {
		return nil
	}

	path = strings.Trim(path, "/")
	segments := strings.Split(path, "/")

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
			return nil
		}

		currentParentID = &folder.ID
	}

	return currentParentID
}
