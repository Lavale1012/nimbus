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

func CreateToken(email, userID string) (string, error) {
	secret, err := utils.GetEnv("JWT_SECRET")
	if err != nil {
		return "", err
	}
	secretKey = []byte(secret)
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

func VerifyToken(tokenString string) error {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
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

func GetEmailFromToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
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

func AuthenticateUser(c *gin.Context, db *gorm.DB) (*models.UserModel, error) {
	authToken := c.GetHeader("Authorization")
	if authToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization token"})
		return nil, fmt.Errorf("missing token")
	}

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

	var user models.UserModel
	if err := db.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return nil, err
	}

	return &user, nil
}
