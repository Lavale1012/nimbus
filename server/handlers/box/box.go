package box

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
	"github.com/nimbus/api/middleware/jwt"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"gorm.io/gorm"
)

const (
	MAX_BOX_NAME = 100
	MIN_BOX_NAME = 1
)

func CreateBox(h s3db.Config, c *gin.Context, db *gorm.DB) {
	user, err := jwt.AuthenticateUser(c, db)
	var existing models.Box

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if h.Bucket == "" || h.Client == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "S3 client or bucket not configured"})
		return
	}

	boxName := c.Query("box_name")
	if boxName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "box name is required"})
		return
	}

	if len(boxName) < MIN_BOX_NAME || len(boxName) > MAX_BOX_NAME {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("box name must be between %d and %d characters", MIN_BOX_NAME, MAX_BOX_NAME)})
		return
	}

	// Sanitize: strip path traversal and replace spaces
	sanitizedName := filepath.Base(boxName)
	sanitizedName = strings.ReplaceAll(sanitizedName, " ", "_")

	// Check for duplicate box name under this user
	if err := db.Where("name = ? AND user_id = ?", sanitizedName, user.ID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a box with that name already exists"})
		return
	}

	// Generate a secure unique BoxID
	boxID, err := utils.GenerateSecureID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate box ID"})
		return
	}

	key := fmt.Sprintf("users/nim-user-%d/boxes/%s/", user.ID, sanitizedName)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	_, err = h.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &h.Bucket,
		Key:    &key,
		Body:   strings.NewReader(""),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create box in storage"})
		return
	}

	if err := db.Create(&models.Box{
		Name:   sanitizedName,
		UserID: user.ID,
		BoxID:  boxID,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save box to database"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "box created successfully", "box": sanitizedName})
}

func DeleteBox(h s3db.Config, c *gin.Context, db *gorm.DB) {
	user, err := jwt.AuthenticateUser(c, db)
	var box models.Box

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	boxName := c.Query("box_name")
	if boxName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "box name is required"})
		return
	}

	sanitizedName := filepath.Base(boxName)
	sanitizedName = strings.ReplaceAll(sanitizedName, " ", "_")

	if err := db.Where("name = ? AND user_id = ?", sanitizedName, user.ID).First(&box).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "box not found"})
		return
	}

	key := fmt.Sprintf("users/nim-user-%d/boxes/%s/", user.ID, sanitizedName)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	_, err = h.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &h.Bucket,
		Key:    &key,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete box from storage"})
		return
	}

	if err := db.Delete(&box).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete box from database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "box deleted successfully"})
}
func ListBoxes(h s3db.Config, c *gin.Context, db *gorm.DB) {
	user, err := jwt.AuthenticateUser(c, db)
	var boxes []models.Box

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := db.Where("user_id = ?", user.ID).Find(&boxes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list boxes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"boxes": boxes})
}
