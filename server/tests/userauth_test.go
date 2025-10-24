package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	userhandlers "github.com/nimbus/api/handlers/userHandlers"
	"github.com/nimbus/api/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(&models.UserModel{}, &models.BoxModel{}, &models.FolderModel{}, &models.FileModel{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

// TestUserRegister_Success tests successful user registration
func TestUserRegister_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)

	router := gin.New()
	router.POST("/register", func(c *gin.Context) {
		userhandlers.UserRegister(c, db)
	})

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "Test123!@#",
		"passkey":  "1234",
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "User registered successfully", response["message"])
	assert.Equal(t, "test@example.com", response["email"])
}

// TestUserRegister_MissingFields tests registration with missing required fields
func TestUserRegister_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)

	router := gin.New()
	router.POST("/register", func(c *gin.Context) {
		userhandlers.UserRegister(c, db)
	})

	testCases := []struct {
		name     string
		reqBody  map[string]string
		expected string
	}{
		{
			name:     "Missing email",
			reqBody:  map[string]string{"password": "Test123!@#", "passkey": "1234"},
			expected: "Email, password, and passkey are required",
		},
		{
			name:     "Missing password",
			reqBody:  map[string]string{"email": "test@example.com", "passkey": "1234"},
			expected: "Email, password, and passkey are required",
		},
		{
			name:     "Missing passkey",
			reqBody:  map[string]string{"email": "test@example.com", "password": "Test123!@#"},
			expected: "Email, password, and passkey are required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.reqBody)
			req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			assert.Equal(t, tc.expected, response["error"])
		})
	}
}

// TestUserRegister_InvalidEmail tests registration with invalid email formats
func TestUserRegister_InvalidEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)

	router := gin.New()
	router.POST("/register", func(c *gin.Context) {
		userhandlers.UserRegister(c, db)
	})

	invalidEmails := []string{
		"notanemail",
		"@example.com",
		"test@",
		"test@@example.com",
		"test@.com",
	}

	for _, email := range invalidEmails {
		t.Run(email, func(t *testing.T) {
			reqBody := map[string]string{
				"email":    email,
				"password": "Test123!@#",
				"passkey":  "1234",
			}
			body, _ := json.Marshal(reqBody)
			req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			assert.Equal(t, "Invalid email format", response["error"])
		})
	}
}

// TestUserRegister_WeakPassword tests registration with weak passwords
func TestUserRegister_WeakPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)

	router := gin.New()
	router.POST("/register", func(c *gin.Context) {
		userhandlers.UserRegister(c, db)
	})

	weakPasswords := []struct {
		password string
		reason   string
	}{
		{"short", "too short (less than 8 characters)"},
		{"nouppercase123!", "missing uppercase letter"},
		{"NOLOWERCASE123!", "missing lowercase letter"},
		{"NoNumbers!@#", "missing number"},
		{"NoSpecial123", "missing special character"},
		{"1234567", "too short and missing other requirements"},
	}

	for _, tc := range weakPasswords {
		t.Run(tc.reason, func(t *testing.T) {
			reqBody := map[string]string{
				"email":    "test@example.com",
				"password": tc.password,
				"passkey":  "1234",
			}
			body, _ := json.Marshal(reqBody)
			req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestUserRegister_DuplicateEmail tests that duplicate emails are rejected
func TestUserRegister_DuplicateEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)

	router := gin.New()
	router.POST("/register", func(c *gin.Context) {
		userhandlers.UserRegister(c, db)
	})

	reqBody := map[string]string{
		"email":    "duplicate@example.com",
		"password": "Test123!@#",
		"passkey":  "1234",
	}

	// First registration - should succeed
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Second registration with same email - should fail
	body, _ = json.Marshal(reqBody)
	req, _ = http.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "User already exists", response["error"])
}

// TestUserRegister_PasswordTooLong tests that extremely long passwords are rejected
func TestUserRegister_PasswordTooLong(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)

	router := gin.New()
	router.POST("/register", func(c *gin.Context) {
		userhandlers.UserRegister(c, db)
	})

	// Create a password longer than 72 characters
	longPassword := "Test123!@#" + string(make([]byte, 70))

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": longPassword,
		"passkey":  "1234",
	}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Password exceeds maximum allowed length", response["error"])
}

// TestUserRegister_HomeBoxCreation tests that a home box is created for new users
func TestUserRegister_HomeBoxCreation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)

	router := gin.New()
	router.POST("/register", func(c *gin.Context) {
		userhandlers.UserRegister(c, db)
	})

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "Test123!@#",
		"passkey":  "1234",
	}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify that a box was created for the user
	var user models.UserModel
	db.Preload("Boxes").Where("email = ?", "test@example.com").First(&user)

	assert.NotZero(t, user.ID)
	assert.Equal(t, 1, len(user.Boxes))
	assert.Equal(t, "Home-Box", user.Boxes[0].Name)
	assert.NotZero(t, user.Boxes[0].BoxID)
}
