package userhandlers

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	jwt "github.com/nimbus/api/middleware/auth/JWT"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"gorm.io/gorm"
)

const (
	MAX_EMAIL_LENGTH    = 254
	MAX_PASSWORD_LENGTH = 72
	MIN_PASSWORD_LENGTH = 8
	PASSKEY_LENGTH      = 4
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func isValidPassword(s string) (minLength, number, upper, lower, special bool) {
	var hasNumber, hasUpper, hasLower, hasSpecial bool

	for _, c := range s {
		switch {
		case unicode.IsNumber(c):
			hasNumber = true
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSpecial = true
		}
	}

	minLength = len(s) >= 8
	number = hasNumber
	upper = hasUpper
	lower = hasLower
	special = hasSpecial
	return
}

func isEmailValid(e string) bool {
	emailRegex := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	return emailRegex.MatchString(e)
}
func UserLogin(c *gin.Context, db *gorm.DB) {
	var user models.UserModel

	var loginRequest LoginRequest
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if loginRequest.Email == "" || loginRequest.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
		return
	}

	if len(loginRequest.Email) > MAX_EMAIL_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if !isEmailValid(loginRequest.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	//Query user by email
	err := db.Where("email = ?", loginRequest.Email).First(&user).Error
	var isValid bool

	if err != nil {
		// User not found - do dummy hash check to maintain constant time
		dummyHash := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
		utils.VerifyPasswordHash(loginRequest.Password, dummyHash)
		isValid = false
	} else {
		isValid = utils.VerifyPasswordHash(loginRequest.Password, user.Password)
	}

	if !isValid {
		log.Printf("Failed login attempt for email: %s from IP: %s ", loginRequest.Email, c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Generate JWT token
	token, err := jwt.CreateToken(user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Login successful", "token": token})
}

func UserRegister(c *gin.Context, db *gorm.DB, s3Client *s3.Client) {

	var user models.UserModel

	// Step 1: Bind JSON from request
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Step 2: Validate required fields
	if user.Email == "" || user.Password == "" || user.PassKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email, password, and passkey are required"})
		return
	}

	// Step 3: Validate email format
	if !isEmailValid(user.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	// Step 4: Validate input length constraints (before hashing)
	if len(user.Email) > MAX_EMAIL_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email exceeds maximum allowed length"})
		return
	}

	if len(user.Password) < MIN_PASSWORD_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters long"})
		return
	}

	if len(user.Password) > MAX_PASSWORD_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password exceeds maximum allowed length"})
		return
	}

	if len(user.PassKey) > PASSKEY_LENGTH || len(user.PassKey) < PASSKEY_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passkey must be exactly 4 characters long"})
		return
	}

	// Step 5: Validate password strength (on plain text)
	minLength, number, upper, lower, special := isValidPassword(user.Password)
	if !minLength || !number || !upper || !lower || !special {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Password must be at least 8 characters and include at least one number, one uppercase letter, one lowercase letter, and one special character",
		})
		return
	}

	// Step 6: Check if email already exists
	var existingUser models.UserModel
	if err := db.Where("email = ?", user.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// Step 7: Hash password
	hashedPassword, err := utils.PasswordHash(user.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}
	user.Password = hashedPassword

	// Step 8: Hash passkey (should also be hashed for security)
	hashedPassKey, err := utils.PasswordHash(user.PassKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}
	user.PassKey = hashedPassKey

	// Step 9: Generate random user ID (7-8 digits)
	userID, err := utils.GenerateUserID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}

	// Check for ID collision (very rare but possible)
	var existingUserByID models.UserModel
	for {
		if err := db.First(&existingUserByID, userID).Error; err != nil {
			// ID doesn't exist, we can use it
			break
		}
		// ID collision, generate a new one
		userID, err = utils.GenerateUserID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
			return
		}
	}
	user.ID = userID

	// Step 10: Generate secure BoxID for home box
	boxID, err := utils.GenerateSecureID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}

	// Step 11: Create user with home box
	user.Boxes = []models.BoxModel{{
		Name:  "Home-Box",
		BoxID: boxID,
	}}

	// Step 12: Create user in database with the generated ID
	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	// Step 13: Set bucket name after user creation (now we have the user.ID)
	user.BucketPrefix = fmt.Sprintf("users/nim-user-%d/boxes/%s/files/", user.ID, user.Boxes[0].Name)

	// Update user with bucket name
	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user bucket"})
		return
	}

	// Step 14: Return success with user ID
	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"email":   user.Email,
		"user_id": user.ID,
	})
}

func UserLogout(c *gin.Context) {
	// TODO: Implementation for user logout
}
