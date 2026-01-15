package filehandlers

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	config "github.com/nimbus/api/db/S3/config"
	s3Operations "github.com/nimbus/api/db/S3/operations"
	jwt "github.com/nimbus/api/middleware/auth/JWT"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils/helpers"
	"gorm.io/gorm"
)

func DownloadFile(d config.AWS3ConfigFile, db *gorm.DB, c *gin.Context) {
	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		return // Error response already sent by authenticateUser
	}

	if d.S3 == nil || d.Bucket == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "downloader not configured: missing S3 client or bucket"})
		return
	}

	BoxName := c.Query("box_name")
	if BoxName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "box name is required"})
		return
	}
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file key is required"})
		return
	}
	box, err := helpers.ValidateBoxOwnership(db, BoxName, user.ID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	keyPath := fmt.Sprintf("users/nim-user-%v/boxes/%s/%s", user.ID, box.Name, key)
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
		Key:    &keyPath,
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
	filepath := c.Query("filePath")
	// 1. Authenticate user from JWT token
	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		return // Error response already sent by authenticateUser
	}

	// 2. Validate S3 configuration
	if h.S3 == nil || h.Bucket == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "S3 not configured"})
		return
	}

	// 3. Parse and validate box_id
	// boxID, err := helpers.ParseBoxID(c)
	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	// 	return
	// }
	boxName := c.Query("box_name")
	// 4. Get and validate uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file input error: " + err.Error()})
		return
	}
	defer file.Close()

	if header == nil || header.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file"})
		return
	}

	// 5. Verify box ownership
	box, err := helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	// 6. Generate unique S3 key
	s3Key, err := helpers.GenerateS3Key(filepath, header.Filename, boxName, user)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 7. Determine content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 8. Upload to S3
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	if err := s3Operations.PutObject(ctx, h.S3, h.Bucket, s3Key, contentType, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload to S3: " + err.Error()})
		return
	}

	// 9. Save file metadata to database
	fileModel := &models.FileModel{
		UserID: user.ID,
		BoxID:  box.ID,
		Name:   header.Filename,
		Size:   header.Size,
	}

	if err := db.Create(fileModel).Error; err != nil {
		// TODO: Delete from S3 to rollback
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to save file metadata",
			"details": err.Error(),
		})
		return
	}

	// 10. Optionally associate with folder
	helpers.AssociateFileWithFolder(db, c, fileModel, box.ID)

	// 11. Return success response
	c.JSON(http.StatusOK, gin.H{
		"message": "file uploaded successfully",
		"file_id": fileModel.ID,
		"name":    fileModel.Name,
		"size":    fileModel.Size,
		"s3_key":  s3Key,
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
