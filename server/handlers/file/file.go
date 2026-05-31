package file

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
	"github.com/nimbus/api/middleware/jwt"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils/helpers"
	"gorm.io/gorm"
)

const presignExpiry = 15 * time.Minute

func PresignDownload(d s3db.Config, c *gin.Context, db *gorm.DB) {
	startTime := time.Now()

	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		log.Printf("[PRESIGN-DOWNLOAD] Auth failed from IP: %s", c.ClientIP())
		return
	}

	if d.Client == nil || d.Bucket == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "S3 not configured"})
		return
	}

	boxName := c.Query("box_name")
	if boxName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "box_name is required"})
		return
	}

	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
		return
	}

	box, err := helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		log.Printf("[PRESIGN-DOWNLOAD] Access denied - user_id: %d, box: %s", user.ID, boxName)
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	s3Key := fmt.Sprintf("users/nim-user-%v/boxes/%s/%s", user.ID, box.Name, key)

	var fileModel models.File
	if err := db.Where("s3_key = ? AND user_id = ?", s3Key, user.ID).First(&fileModel).Error; err != nil {
		log.Printf("[PRESIGN-DOWNLOAD] File not found - user_id: %d, key: %s", user.ID, s3Key)
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	url, err := s3db.PresignGetObject(ctx, d.Client, d.Bucket, s3Key, presignExpiry)
	if err != nil {
		log.Printf("[PRESIGN-DOWNLOAD] Presign failed - user_id: %d, key: %s, error: %v", user.ID, s3Key, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate download URL"})
		return
	}

	log.Printf("[PRESIGN-DOWNLOAD] Success - user_id: %d, file: %s, duration: %v", user.ID, fileModel.Name, time.Since(startTime))
	c.JSON(http.StatusOK, gin.H{"download_url": url, "expires_in": presignExpiry.String()})
}

func PresignUpload(h s3db.Config, db *gorm.DB, c *gin.Context) {
	startTime := time.Now()

	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		log.Printf("[PRESIGN-UPLOAD] Auth failed from IP: %s", c.ClientIP())
		return
	}

	if h.Client == nil || h.Bucket == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "S3 not configured"})
		return
	}

	boxName := c.Query("box_name")
	filePath := c.Query("filePath")
	filename := c.Query("filename")
	contentType := c.Query("content_type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filename is required"})
		return
	}

	var fileSize int64
	fmt.Sscanf(c.Query("size"), "%d", &fileSize)

	box, err := helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		log.Printf("[PRESIGN-UPLOAD] Access denied - user_id: %d, box: %s", user.ID, boxName)
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	s3Key, err := helpers.GenerateS3Key(filePath, filename, boxName, user)
	if err != nil {
		log.Printf("[PRESIGN-UPLOAD] Key generation failed - user_id: %d, error: %v", user.ID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fileModel := &models.File{
		UserID: user.ID,
		BoxID:  box.ID,
		Name:   filename,
		Size:   fileSize,
		S3Key:  s3Key,
	}
	if err := db.Create(fileModel).Error; err != nil {
		log.Printf("[PRESIGN-UPLOAD] DB save failed - user_id: %d, file: %s, error: %v", user.ID, filename, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file metadata"})
		return
	}

	helpers.AssociateFileWithFolder(db, c, fileModel, box.ID)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	url, err := s3db.PresignPutObject(ctx, h.Client, h.Bucket, s3Key, contentType, presignExpiry)
	if err != nil {
		db.Delete(fileModel)
		log.Printf("[PRESIGN-UPLOAD] Presign failed - user_id: %d, key: %s, error: %v", user.ID, s3Key, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate upload URL"})
		return
	}

	log.Printf("[PRESIGN-UPLOAD] Success - user_id: %d, file: %s, duration: %v", user.ID, filename, time.Since(startTime))
	c.JSON(http.StatusOK, gin.H{
		"upload_url": url,
		"s3_key":     s3Key,
		"file_id":    fileModel.ID,
		"expires_in": presignExpiry.String(),
	})
}

func Delete(d s3db.Config, db *gorm.DB, c *gin.Context) {
	startTime := time.Now()

	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		log.Printf("[DELETE] Auth failed from IP: %s", c.ClientIP())
		return
	}

	keyName := c.Param("name")
	if keyName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file name is required"})
		return
	}

	if d.Client == nil || d.Bucket == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "S3 not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var fileModel models.File
	if err := db.Where("s3_key = ? AND user_id = ?", keyName, user.ID).First(&fileModel).Error; err != nil {
		log.Printf("[DELETE] File not found - user_id: %d, key: %s", user.ID, keyName)
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found in database"})
		return
	}

	if _, err := d.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &d.Bucket,
		Key:    &keyName,
	}); err != nil {
		log.Printf("[DELETE] S3 delete failed - user_id: %d, key: %s, error: %v", user.ID, keyName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to delete object: %v", err)})
		return
	}

	if err := db.Delete(&fileModel).Error; err != nil {
		log.Printf("[DELETE] DB delete failed - user_id: %d, key: %s, error: %v", user.ID, keyName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete file record"})
		return
	}

	log.Printf("[DELETE] Success - user_id: %d, file: %s, duration: %v", user.ID, keyName, time.Since(startTime))
	c.JSON(http.StatusOK, gin.H{"message": "file deleted"})
}

func List(h s3db.Config, db *gorm.DB, c *gin.Context) {
	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		log.Printf("[LIST] Auth failed from IP: %s", c.ClientIP())
		return
	}

	boxName := c.Query("box_name")
	if boxName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "box_name is required"})
		return
	}

	box, err := helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var files []struct {
		ID        uint   `json:"id"`
		Name      string `json:"name"`
		Size      int64  `json:"size"`
		S3Key     string `json:"s3_key"`
		CreatedAt string `json:"created_at"`
	}
	db.Model(&models.File{}).
		Where("box_id = ? AND user_id = ?", box.ID, user.ID).
		Select("id, name, size, s3_key, created_at").
		Find(&files)

	c.JSON(http.StatusOK, gin.H{"files": files})
}

func Rename(h s3db.Config, db *gorm.DB, c *gin.Context) {
	startTime := time.Now()

	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		log.Printf("[RENAME] Auth failed from IP: %s", c.ClientIP())
		return
	}

	boxName := c.Query("box_name")
	s3Key := c.Query("key")
	newName := c.Query("new_name")

	if boxName == "" || s3Key == "" || newName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "box_name, key, and new_name are required"})
		return
	}

	_, err = helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var fileModel models.File
	if err := db.Where("s3_key = ? AND user_id = ?", s3Key, user.ID).First(&fileModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	if err := db.Model(&fileModel).Update("name", newName).Error; err != nil {
		log.Printf("[RENAME] DB update failed - user_id: %d, key: %s, error: %v", user.ID, s3Key, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rename file"})
		return
	}

	log.Printf("[RENAME] Success - user_id: %d, key: %s, new_name: %s, duration: %v", user.ID, s3Key, newName, time.Since(startTime))
	c.JSON(http.StatusOK, gin.H{"message": "file renamed", "name": newName})
}

func Move(h s3db.Config, db *gorm.DB, c *gin.Context) {
	startTime := time.Now()

	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		log.Printf("[MOVE] Auth failed from IP: %s", c.ClientIP())
		return
	}

	boxName := c.Query("box_name")
	s3Key := c.Query("key")
	targetPath := c.Query("target_path")

	if boxName == "" || s3Key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "box_name and key are required"})
		return
	}

	_, err = helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var fileModel models.File
	if err := db.Where("s3_key = ? AND user_id = ?", s3Key, user.ID).First(&fileModel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	// Resolve destination folder ID from target_path
	newFolderID := helpers.GetParentFolderID(db, user.ID, boxName, targetPath)
	if err := db.Model(&fileModel).Update("folder_id", newFolderID).Error; err != nil {
		log.Printf("[MOVE] DB update failed - user_id: %d, key: %s, error: %v", user.ID, s3Key, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to move file"})
		return
	}

	log.Printf("[MOVE] Success - user_id: %d, key: %s, target: %s, duration: %v", user.ID, s3Key, targetPath, time.Since(startTime))
	c.JSON(http.StatusOK, gin.H{"message": "file moved"})
}
