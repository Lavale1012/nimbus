// Package user contains HTTP handlers for user registration and login.
package user

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/nimbus/api/middleware/jwt"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"gorm.io/gorm"
)

// Input length limits. MAX_PASSWORD_LENGTH is 72 because bcrypt silently
// truncates passwords longer than 72 bytes — we reject them upfront so users
// aren't surprised when a 73-character password works as a 72-character one.
const (
	MAX_EMAIL_LENGTH    = 254
	MAX_PASSWORD_LENGTH = 72
	MIN_PASSWORD_LENGTH = 8
	PASSKEY_LENGTH      = 4
)

// LoginRequest is the JSON body expected by the /login endpoint.
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// isValidPassword checks that the password meets complexity requirements:
// minimum length, at least one number, one uppercase, one lowercase, and one
// special character. Returns individual booleans so the caller can form a
// specific error message.
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

// isEmailValid uses a regex to check basic email format.
func isEmailValid(e string) bool {
	emailRegex := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	return emailRegex.MatchString(e)
}

// Login validates credentials and returns a signed JWT on success.
// If the email doesn't exist we still run bcrypt on a dummy hash so the
// response time is the same as a real password mismatch — this prevents an
// attacker from enumerating valid emails via timing differences.
func Login(c *gin.Context, db *gorm.DB) {
	var user models.User
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

	if len(loginRequest.Password) > MAX_PASSWORD_LENGTH {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if !isEmailValid(loginRequest.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	// Query user and preload their boxes in one DB round-trip.
	err := db.Preload("Boxes").Where("email = ?", loginRequest.Email).First(&user).Error
	var isValid bool

	if err != nil {
		// User not found — run a dummy bcrypt check so this branch takes the
		// same time as a real failed login, preventing email enumeration.
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

	// Issue a JWT containing the user's email and ID.
	token, err := jwt.CreateToken(user.Email, fmt.Sprintf("%d", user.ID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Login successful", "token": token, "user_id": user.ID, "email": user.Email, "box": user.Boxes})
}

// Register creates a new user account. The registration flow is:
//  1. Validate and sanitize all input fields
//  2. Check for duplicate email
//  3. Hash the password and passkey with bcrypt
//  4. Generate a random 8-digit user ID (retrying on the rare collision)
//  5. Create the user record along with their default "Home-Box"
func Register(c *gin.Context, db *gorm.DB, s3Client *s3.Client) {
	var user models.User

	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if user.Email == "" || user.Password == "" || user.PassKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email, password, and passkey are required"})
		return
	}

	if !isEmailValid(user.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

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

	// Validate password complexity before hashing.
	minLength, number, upper, lower, special := isValidPassword(user.Password)
	if !minLength || !number || !upper || !lower || !special {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Password must be at least 8 characters and include at least one number, one uppercase letter, one lowercase letter, and one special character",
		})
		return
	}

	// Reject duplicate emails before doing any expensive work.
	var existingUser models.User
	if err := db.Where("email = ?", user.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	hashedPassword, err := utils.PasswordHash(user.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}
	user.Password = hashedPassword

	hashedPassKey, err := utils.PasswordHash(user.PassKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}
	user.PassKey = hashedPassKey

	// Generate a random 8-digit user ID. Loop in case we collide with an
	// existing ID (extremely unlikely but theoretically possible).
	userID, err := utils.GenerateUserID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}

	var existingUserByID models.User
	for {
		if err := db.First(&existingUserByID, userID).Error; err != nil {
			break // ID is free to use
		}
		userID, err = utils.GenerateUserID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
			return
		}
	}
	user.ID = userID

	// Generate a secure random BoxID for the default "Home-Box".
	boxID, err := utils.GenerateSecureID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process registration"})
		return
	}

	// Create the user and their Home-Box in a single DB transaction (GORM handles this via association).
	user.Boxes = []models.Box{{
		Name:  "Home-Box",
		BoxID: boxID,
	}}

	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user bucket"})
		return
	}

	// Return only non-sensitive fields in the response.
	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"email":   user.Email,
		"user_id": user.ID,
		"box":     user.Boxes[0].Name,
	})
}
