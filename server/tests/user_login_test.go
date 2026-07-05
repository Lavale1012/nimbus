package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nimbus/api/handlers/user"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupLoginDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Box{}, &models.Folder{}, &models.File{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func seedLoginUser(t *testing.T, db *gorm.DB, email, password string) *models.User {
	t.Helper()
	hash, _ := utils.PasswordHash(password)
	passKeyHash, _ := utils.PasswordHash("1234")
	userID, _ := utils.GenerateUserID()
	boxID, _ := utils.GenerateSecureID()
	u := &models.User{
		ID:       userID,
		Email:    email,
		Password: hash,
		PassKey:  passKeyHash,
		Boxes:    []models.Box{{Name: "Home-Box", BoxID: boxID}},
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return u
}

func loginRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", func(c *gin.Context) {
		user.Login(c, db)
	})
	return r
}

func TestLogin_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-0123456789-abcdefghij")
	db := setupLoginDB(t)
	seedLoginUser(t, db, "login@example.com", "Test123!@#")
	r := loginRouter(db)

	body, _ := json.Marshal(map[string]string{"email": "login@example.com", "password": "Test123!@#"})
	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "Login successful", resp["message"])
	assert.NotEmpty(t, resp["token"])
	assert.Equal(t, "login@example.com", resp["email"])
}

func TestLogin_WrongPassword(t *testing.T) {
	db := setupLoginDB(t)
	seedLoginUser(t, db, "login2@example.com", "Test123!@#")
	r := loginRouter(db)

	body, _ := json.Marshal(map[string]string{"email": "login2@example.com", "password": "WrongPass123!@#"})
	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "Invalid email or password", resp["error"])
}

func TestLogin_UnknownEmail(t *testing.T) {
	db := setupLoginDB(t)
	r := loginRouter(db)

	body, _ := json.Marshal(map[string]string{"email": "ghost@example.com", "password": "Test123!@#"})
	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "Invalid email or password", resp["error"])
}

func TestLogin_MissingFields(t *testing.T) {
	db := setupLoginDB(t)
	r := loginRouter(db)

	cases := []map[string]string{
		{"email": ""},
		{"password": "Test123!@#"},
		{},
	}

	for _, payload := range cases {
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusUnauthorized,
			"missing fields should return 400 or 401, got %d", w.Code)
	}
}

func TestLogin_InvalidEmailFormat(t *testing.T) {
	db := setupLoginDB(t)
	r := loginRouter(db)

	body, _ := json.Marshal(map[string]string{"email": "notanemail", "password": "Test123!@#"})
	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "Invalid email format", resp["error"])
}

func TestLogin_ReturnsBoxes(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-0123456789-abcdefghij")
	db := setupLoginDB(t)
	seedLoginUser(t, db, "boxes@example.com", "Test123!@#")
	r := loginRouter(db)

	body, _ := json.Marshal(map[string]string{"email": "boxes@example.com", "password": "Test123!@#"})
	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	boxes, ok := resp["box"].([]interface{})
	assert.True(t, ok, "response should include boxes array")
	assert.Equal(t, 1, len(boxes))
}

func TestLogin_CaseSensitiveEmail(t *testing.T) {
	db := setupLoginDB(t)
	seedLoginUser(t, db, "case@example.com", "Test123!@#")
	r := loginRouter(db)

	body, _ := json.Marshal(map[string]string{"email": "CASE@EXAMPLE.COM", "password": "Test123!@#"})
	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Email lookup is case-sensitive (raw SQL WHERE clause), so uppercase should not match
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
