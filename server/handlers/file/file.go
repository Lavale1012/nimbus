package file

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
	"github.com/nimbus/api/middleware/jwt"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils/helpers"
	"gorm.io/gorm"
)

func Download(d s3db.Config, c *gin.Context, db *gorm.DB) {
	startTime := time.Now()

	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		log.Printf("[DOWNLOAD] Auth failed from IP: %s", c.ClientIP())
		return
	}

	if d.Client == nil || d.Bucket == "" {
		log.Printf("[DOWNLOAD] S3 not configured - user_id: %d", user.ID)
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
		log.Printf("[DOWNLOAD] Access denied - user_id: %d, box: %s, IP: %s", user.ID, BoxName, c.ClientIP())
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	keyPath := fmt.Sprintf("users/nim-user-%v/boxes/%s/%s", user.ID, box.Name, key)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	fileName := filepath.Base(keyPath)

	// Check if file exists in Postgres before downloading from S3
	var fileModel models.File
	result := db.Where("s3_key = ? AND user_id = ?", fileName, user.ID).First(&fileModel)
	if result.Error != nil {
		log.Printf("[DOWNLOAD] File not found - user_id: %d, key: %s", user.ID, fileName)
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found in database"})
		return
	}

	// Proceed with downloading the file from S3
	obj, err := d.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &d.Bucket,
		Key:    &keyPath,
	})
	if err != nil {
		log.Printf("[DOWNLOAD] S3 error - user_id: %d, key: %s, error: %v", user.ID, keyPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get object: %v", err)})
		return
	}
	if obj == nil {
		log.Printf("[DOWNLOAD] S3 object nil - user_id: %d, key: %s", user.ID, keyPath)
		c.JSON(http.StatusNotFound, gin.H{"error": "object not found"})
		return
	}
	defer obj.Body.Close()

	ct := "application/octet-stream"
	if obj.ContentType != nil && *obj.ContentType != "" {
		ct = *obj.ContentType
	}

	contentLength := int64(0)
	if obj.ContentLength != nil {
		contentLength = *obj.ContentLength
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Header("Content-Type", ct)
	c.DataFromReader(http.StatusOK, contentLength, ct, obj.Body, nil)

	log.Printf("[DOWNLOAD] Success - user_id: %d, file: %s, size: %d bytes, duration: %v",
		user.ID, fileName, contentLength, time.Since(startTime))
}

func Upload(h s3db.Config, db *gorm.DB, c *gin.Context) {
	startTime := time.Now()
	filePath := c.Query("filePath")

	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		log.Printf("[UPLOAD] Auth failed from IP: %s", c.ClientIP())
		return
	}

	if h.Client == nil || h.Bucket == "" {
		log.Printf("[UPLOAD] S3 not configured - user_id: %d", user.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "S3 not configured"})
		return
	}

	boxName := c.Query("box_name")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		log.Printf("[UPLOAD] File input error - user_id: %d, error: %v", user.ID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "file input error: " + err.Error()})
		return
	}
	defer file.Close()

	if header == nil || header.Size <= 0 {
		log.Printf("[UPLOAD] Invalid file - user_id: %d", user.ID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file"})
		return
	}

	box, err := helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		log.Printf("[UPLOAD] Access denied - user_id: %d, box: %s, IP: %s", user.ID, boxName, c.ClientIP())
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	s3Key, err := helpers.GenerateS3Key(filePath, header.Filename, boxName, user)
	if err != nil {
		log.Printf("[UPLOAD] Key generation failed - user_id: %d, error: %v", user.ID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	if err := s3db.PutObject(ctx, h.Client, h.Bucket, s3Key, contentType, file); err != nil {
		log.Printf("[UPLOAD] S3 upload failed - user_id: %d, key: %s, error: %v", user.ID, s3Key, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload to S3: " + err.Error()})
		return
	}

	fileModel := &models.File{
		UserID: user.ID,
		BoxID:  box.ID,
		Name:   header.Filename,
		Size:   header.Size,
		S3Key:  s3Key,
	}

	if err := db.Create(fileModel).Error; err != nil {
		log.Printf("[UPLOAD] DB save failed - user_id: %d, file: %s, error: %v", user.ID, header.Filename, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to save file metadata",
			"details": err.Error(),
		})
		return
	}

	helpers.AssociateFileWithFolder(db, c, fileModel, box.ID)

	log.Printf("[UPLOAD] Success - user_id: %d, file: %s, size: %d bytes, duration: %v",
		user.ID, header.Filename, header.Size, time.Since(startTime))

	c.JSON(http.StatusOK, gin.H{
		"message": "file uploaded successfully",
		"file_id": fileModel.ID,
		"name":    fileModel.Name,
		"size":    fileModel.Size,
		"s3_key":  s3Key,
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
		log.Printf("[DELETE] S3 not configured - user_id: %d", user.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "S3 not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Check if file exists and belongs to user
	var fileModel models.File
	result := db.Where("s3_key = ? AND user_id = ?", keyName, user.ID).First(&fileModel)
	if result.Error != nil {
		log.Printf("[DELETE] File not found - user_id: %d, key: %s", user.ID, keyName)
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found in database"})
		return
	}

	_, err = d.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &d.Bucket,
		Key:    &keyName,
	})
	if err != nil {
		log.Printf("[DELETE] S3 delete failed - user_id: %d, key: %s, error: %v", user.ID, keyName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to delete object: %v", err)})
		return
	}

	result = db.Delete(&fileModel)
	if result.Error != nil {
		log.Printf("[DELETE] DB delete failed - user_id: %d, key: %s, error: %v", user.ID, keyName, result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete file record from database"})
		return
	}

	log.Printf("[DELETE] Success - user_id: %d, file: %s, duration: %v",
		user.ID, keyName, time.Since(startTime))

	c.JSON(http.StatusOK, gin.H{"message": "file deleted"})
}

func List(h s3db.Config, db *gorm.DB, c *gin.Context)   {}
func Move(h s3db.Config, db *gorm.DB, c *gin.Context)   {}
func Rename(h s3db.Config, db *gorm.DB, c *gin.Context) {}
