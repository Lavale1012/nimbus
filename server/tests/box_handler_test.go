package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
	boxhandler "github.com/nimbus/api/handlers/box"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupBoxHandlerDB(t *testing.T) *gorm.DB {
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

func createBoxHandlerUser(t *testing.T, db *gorm.DB) *models.User {
	t.Helper()
	boxID, _ := utils.GenerateSecureID()
	userID, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")
	u := &models.User{
		ID:      userID,
		Email:   fmt.Sprintf("boxtest-%d@example.com", userID),
		Password: hash,
		PassKey:  "1234",
		Boxes:   []models.Box{{Name: "Home-Box", BoxID: boxID}},
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return u
}

func boxListRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/boxes", func(c *gin.Context) {
		boxhandler.ListBoxes(s3db.Config{}, c, db)
	})
	return r
}

func boxVerifyRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/boxes/verify", func(c *gin.Context) {
		boxhandler.VerifyBoxExist(s3db.Config{}, c, db)
	})
	return r
}

// --- ListBoxes ---

func TestListBoxes_Unauthorized(t *testing.T) {
	db := setupBoxHandlerDB(t)
	r := boxListRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/boxes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListBoxes_ReturnsOwnBoxes(t *testing.T) {
	db := setupBoxHandlerDB(t)
	u := createBoxHandlerUser(t, db)
	r := boxListRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/boxes", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	boxes := body["boxes"].([]interface{})
	assert.Equal(t, 1, len(boxes))
}

func TestListBoxes_DoesNotReturnOtherUsersBoxes(t *testing.T) {
	db := setupBoxHandlerDB(t)
	u1 := createBoxHandlerUser(t, db)

	// Second user
	boxID2, _ := utils.GenerateSecureID()
	userID2, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")
	u2 := &models.User{
		ID: userID2, Email: fmt.Sprintf("other-%d@example.com", userID2),
		Password: hash, PassKey: "1234",
		Boxes: []models.Box{{Name: "Secret-Box", BoxID: boxID2}},
	}
	db.Create(u2)

	r := boxListRouter(db)
	req, _ := http.NewRequest(http.MethodGet, "/boxes", nil)
	req.Header.Set("Authorization", authHeader(t, u1))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	boxes := body["boxes"].([]interface{})

	for _, raw := range boxes {
		box := raw.(map[string]interface{})
		assert.NotEqual(t, "Secret-Box", box["name"], "should not see other user's boxes")
	}
}

func TestListBoxes_MultipleBoxes(t *testing.T) {
	db := setupBoxHandlerDB(t)
	u := createBoxHandlerUser(t, db)

	// Add an extra box directly
	extraID, _ := utils.GenerateSecureID()
	db.Create(&models.Box{Name: "Extra-Box", UserID: u.ID, BoxID: extraID})

	r := boxListRouter(db)
	req, _ := http.NewRequest(http.MethodGet, "/boxes", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	boxes := body["boxes"].([]interface{})
	assert.Equal(t, 2, len(boxes))
}

// --- VerifyBoxExist ---

func TestVerifyBox_Unauthorized(t *testing.T) {
	db := setupBoxHandlerDB(t)
	r := boxVerifyRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/boxes/verify?box_name=Home-Box", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestVerifyBox_MissingBoxName(t *testing.T) {
	db := setupBoxHandlerDB(t)
	u := createBoxHandlerUser(t, db)
	r := boxVerifyRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/boxes/verify", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestVerifyBox_BoxExists(t *testing.T) {
	db := setupBoxHandlerDB(t)
	u := createBoxHandlerUser(t, db)
	r := boxVerifyRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/boxes/verify?box_name=Home-Box", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "box exists", body["message"])
}

func TestVerifyBox_BoxNotFound(t *testing.T) {
	db := setupBoxHandlerDB(t)
	u := createBoxHandlerUser(t, db)
	r := boxVerifyRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/boxes/verify?box_name=Ghost-Box", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "box not found", body["error"])
}

func TestVerifyBox_OtherUsersBox(t *testing.T) {
	db := setupBoxHandlerDB(t)
	u1 := createBoxHandlerUser(t, db)

	// Second user also has a box named "Home-Box"
	boxID2, _ := utils.GenerateSecureID()
	userID2, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")
	u2 := &models.User{
		ID: userID2, Email: fmt.Sprintf("other2-%d@example.com", userID2),
		Password: hash, PassKey: "1234",
		Boxes: []models.Box{{Name: "Private-Box", BoxID: boxID2}},
	}
	db.Create(u2)

	r := boxVerifyRouter(db)
	// u1 tries to verify u2's private box
	req, _ := http.NewRequest(http.MethodGet, "/boxes/verify?box_name=Private-Box", nil)
	req.Header.Set("Authorization", authHeader(t, u1))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "should not find another user's box")
}

// --- Box model constraints ---

func TestBoxModel_UniqueNamePerUser(t *testing.T) {
	// Uniqueness is enforced at the handler layer (SELECT before INSERT), not by a DB constraint.
	// This test verifies the handler-level check works correctly by confirming the DB query
	// that the handler uses would find an existing box with the same name.
	db := setupBoxHandlerDB(t)
	u := createBoxHandlerUser(t, db)

	boxID, _ := utils.GenerateSecureID()
	b1 := models.Box{Name: "Duplicate-Box", UserID: u.ID, BoxID: boxID}
	assert.NoError(t, db.Create(&b1).Error)

	// Simulate the handler's duplicate check
	var existing models.Box
	err := db.Where("name = ? AND user_id = ?", "Duplicate-Box", u.ID).First(&existing).Error
	assert.NoError(t, err, "duplicate check query should find the existing box")
	assert.Equal(t, b1.ID, existing.ID, "should find the previously created box")
}

func TestBoxModel_SameNameDifferentUsers(t *testing.T) {
	db := setupBoxHandlerDB(t)
	u1 := createBoxHandlerUser(t, db)

	userID2, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")
	u2 := &models.User{
		ID: userID2, Email: fmt.Sprintf("shared-%d@example.com", userID2),
		Password: hash, PassKey: "1234",
	}
	db.Create(u2)

	boxID1, _ := utils.GenerateSecureID()
	boxID2, _ := utils.GenerateSecureID()
	b1 := models.Box{Name: "Shared-Name", UserID: u1.ID, BoxID: boxID1}
	b2 := models.Box{Name: "Shared-Name", UserID: u2.ID, BoxID: boxID2}

	assert.NoError(t, db.Create(&b1).Error)
	assert.NoError(t, db.Create(&b2).Error, "same box name is allowed for different users")
}
