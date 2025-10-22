package filehandlers

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	config "github.com/nimbus/api/db/S3/config"
	s3Operations "github.com/nimbus/api/db/S3/operations"
	boxhandlers "github.com/nimbus/api/handlers/boxHandlers"
	folderhandlers "github.com/nimbus/api/handlers/folderHandlers"
	"github.com/nimbus/api/models"
	"gorm.io/gorm"
)

func DownloadFile(d config.AWS3ConfigFile, db *gorm.DB, c *gin.Context) {
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

	//Check if file exists in Postgres before downloading from S3
	var fileModel models.FileModel
	result := db.Model(&models.FileModel{}).Where("s3_key = ?", key).First(&fileModel)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found in database"})
		return
	}

	//TODO: Additional permission checks can be added here (e.g., verify user owns the file)

	//TODO: Check size of file before downloading to prevent large downloads if needed

	//TODO: Implement logging of download activity if required

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

func UploadFile(h config.AWS3ConfigFile, db *gorm.DB, c *gin.Context) {
	// Guard against nil client or empty bucket to avoid panics
	var userID uint
	if h.S3 == nil || h.Bucket == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "uploader not configured: missing S3 client or bucket"})
		return
	}

	// Handle file upload logic here
	file, header, err := c.Request.FormFile("file")

	if err != nil {
		c.JSON(400, gin.H{"error": "File input error: " + err.Error()})
		return
	} else if header == nil {
		c.JSON(400, gin.H{"error": "File header is nil"})
		return
	}

	strUserId := c.Query("user_id")
	intUserID, err := strconv.Atoi(strUserId)
	if err == nil {
		userID = uint(intUserID)
	}

	if err != nil {
		c.JSON(400, gin.H{"error": "File is required"})
		return
	}
	defer file.Close()

	//TODO: Additional permission checks can be added here (e.g., verify user owns the file)

	//TODO: Get UserID from authenticated session
	//TODO: Get BoxID from request parameters or user's default box
	//TODO: Get FolderIDs from request parameters (can be multiple folders)

	boxID := boxhandlers.BoxID

	// Validate that the user exists
	var user models.UserModel
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("User with ID %d not found. Please ensure the user exists in the database.", userID),
		})
		return
	}

	// Validate that the box exists and belongs to the user
	var box models.BoxModel
	if err := db.Where("id = ? AND user_id = ?", boxID, userID).First(&box).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Box with ID %d not found or does not belong to user %d. Please create a box first.", boxID, userID),
		})
		return
	}

	// Create the file record
	fileModel := &models.FileModel{
		UserID: uint(intUserID),
		BoxID:  boxID,
		Name:   header.Filename,
		Size:   header.Size,
		S3Key:  header.Filename,
	}

	result := db.Create(fileModel)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save file metadata: %v", result.Error)})
		return
	}

	// Associate file with folder(s) if FolderID is provided
	// In a real implementation, you'd get folder IDs from the request (e.g., query params or JSON body)
	if folderhandlers.FolderID != 0 {
		var folder models.FolderModel
		if err := db.First(&folder, folderhandlers.FolderID).Error; err == nil {
			// Associate the file with the folder using GORM's many2many
			db.Model(fileModel).Association("Folders").Append(&folder)
		}
	}

	//TODO: Check size of file before uploading to prevent large uploads if needed

	//TODO: Implement logging of upload activity if required

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

func DeleteFile(d config.AWS3ConfigFile, db *gorm.DB, c *gin.Context) {
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
	//Check if file exists in Postgres before deleting from S3
	var fileModel models.FileModel
	result := db.Model(&models.FileModel{}).Where("s3_key = ?", keyName).First(&fileModel)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found in database"})
		return
	}
	_, err := d.S3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &d.Bucket,
		Key:    &keyName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to delete object: %v", err)})
		return
	}
	// Delete file record from Postgres
	result = db.Delete(&fileModel)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete file record from database"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "file deleted"})

}

func ListFiles(h config.AWS3ConfigFile, db *gorm.DB, c *gin.Context)  {}
func MoveFile(h config.AWS3ConfigFile, db *gorm.DB, c *gin.Context)   {}
func RenameFile(h config.AWS3ConfigFile, db *gorm.DB, c *gin.Context) {}
