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
	// Step 1: Validate S3 configuration
	if h.S3 == nil || h.Bucket == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "uploader not configured: missing S3 client or bucket"})
		return
	}

	// Step 2: Get user_id and box_id from URL query parameters
	strUserID := c.Query("user_id")
	fmt.Printf("🔍 DEBUG: Received user_id from Query: '%s'\n", strUserID)

	if strUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required in query parameters"})
		return
	}

	intUserID, err := strconv.Atoi(strUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}
	userID := uint(intUserID)
	fmt.Printf("✅ DEBUG: Parsed user_id: %d\n", userID)

	strBoxID := c.Query("box_id")
	fmt.Printf("🔍 DEBUG: Received box_id from Query: '%s'\n", strBoxID)

	if strBoxID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "box_id is required in query parameters"})
		return
	}

	intBoxID, err := strconv.Atoi(strBoxID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid box_id"})
		return
	}

	boxID := uint(intBoxID)
	fmt.Printf("✅ DEBUG: Parsed box_id: %d\n", boxID)

	// Step 3: Get the file from multipart form data
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File input error: " + err.Error()})
		return
	}
	defer file.Close()

	if header == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File header is nil"})
		return
	}

	size := header.Size
	if size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size must be greater than zero"})
		return
	}

	// Step 4: Validate that the user exists
	var user models.UserModel
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("User with ID %d not found", userID),
		})
		return
	}

	// Step 5: Validate that the box exists and belongs to the user
	var box models.BoxModel
	if err := db.Where("box_id = ? AND user_id = ?", boxID, userID).First(&box).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Box with ID %d not found or does not belong to user %d", boxID, userID),
		})
		return
	}

	// Use the box's actual database ID (not the custom BoxID) for foreign key reference
	actualBoxID := box.ID

	// Step 6: Sanitize the filename and build unique S3 key
	base := filepath.Base(header.Filename)
	base = strings.ReplaceAll(base, " ", "_")
	if base == "" {
		base = "upload.bin"
	}
	// Make key unique to prevent collisions
	timestamp := time.Now().Unix()
	key := fmt.Sprintf("users/%d/boxes/%d/%d_%s", userID, boxID, timestamp, base)

	// Step 7: Get content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Step 8: Upload to S3 FIRST (before database)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	err = s3Operations.PutObject(ctx, h.S3, h.Bucket, key, contentType, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file to S3: " + err.Error()})
		return
	}

	// Step 9: Create the file record in database AFTER successful S3 upload
	fileModel := &models.FileModel{
		UserID: userID,
		BoxID:  actualBoxID, // Use the box's actual database ID, not the custom BoxID
		Name:   header.Filename,
		Size:   size,
		S3Key:  key,
	}

	result := db.Create(fileModel)
	if result.Error != nil {
		// If DB save fails, we should ideally delete from S3 (rollback)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "File uploaded but failed to save metadata",
			"details": result.Error.Error(),
		})
		return
	}

	// Step 10: (Optional) Associate file with folder if folder_id provided

	strFolderID := c.PostForm("folder_id")
	if strFolderID != "" {
		intFolderID, err := strconv.Atoi(strFolderID)
		if err == nil && intFolderID > 0 {
			var folder models.FolderModel
			if err := db.First(&folder, uint(intFolderID)).Error; err == nil {
				// Verify folder belongs to same box
				if folder.BoxID == boxID {
					db.Model(fileModel).Association("Folders").Append(&folder)
				}
			}
		}
	}

	// Step 11: Return success response
	c.JSON(http.StatusOK, gin.H{
		"message": "file uploaded successfully",
		"file_id": fileModel.ID,
		"name":    fileModel.Name,
		"size":    fileModel.Size,
		"s3_key":  key,
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
