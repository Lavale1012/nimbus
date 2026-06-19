// Package jwt handles creating, verifying, and reading JWT tokens used to
// authenticate CLI requests to the API.
package jwt

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"gorm.io/gorm"
)

var secretKey []byte

func init() {
	secret, err := utils.GetEnv("JWT_SECRET")
	if err != nil {
		panic("JWT_SECRET is not set: " + err.Error())
	}
	secretKey = []byte(secret)
}

// CreateToken issues a signed JWT for the given email and userID.
// The token expires after 24 hours and is signed with HS256 using JWT_SECRET.
func CreateToken(email, userID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"user_id": userID,
			"email":   email,
			"exp":     time.Now().Add(time.Hour * 24).Unix(),
		})

	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// VerifyToken checks that tokenString is a valid, unexpired JWT signed with
// JWT_SECRET. It also rejects tokens that weren't signed with HMAC (HS256) to
// prevent the "alg:none" attack where an attacker strips the signature.
func VerifyToken(tokenString string) error {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Reject any token whose header says it uses a non-HMAC algorithm.
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})

	if err != nil {
		return err
	}

	if !token.Valid {
		return fmt.Errorf("invalid token")
	}

	return nil
}

// GetEmailFromToken decodes the token's claims and returns the email field.
// Assumes the token has already been verified with VerifyToken.
func GetEmailFromToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})
	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if email, ok := claims["email"].(string); ok {
			return email, nil
		}
	}
	return "", fmt.Errorf("invalid token claims")
}

// AuthenticateUser is a convenience helper used by every protected handler.
// It reads the "Authorization: Bearer <token>" header, verifies the token,
// extracts the email, and looks up the full User record in the database.
// On any failure it writes the appropriate HTTP error response and returns nil.
func AuthenticateUser(c *gin.Context, db *gorm.DB) (*models.User, error) {
	authToken := c.GetHeader("Authorization")
	if authToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization token"})
		return nil, fmt.Errorf("missing token")
	}

	// Strip the "Bearer " prefix so we're left with just the raw token string.
	authToken = strings.TrimPrefix(authToken, "Bearer ")

	if err := VerifyToken(authToken); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return nil, err
	}

	email, err := GetEmailFromToken(authToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "failed to extract email from token"})
		return nil, err
	}

	var user models.User
	if err := db.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return nil, err
	}

	return &user, nil
}
