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

func List(h s3db.Config, db *gorm.DB, c *gin.Context)   {}
func Move(h s3db.Config, db *gorm.DB, c *gin.Context)   {}
func Rename(h s3db.Config, db *gorm.DB, c *gin.Context) {}
