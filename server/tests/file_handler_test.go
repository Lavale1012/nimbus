package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
	filehandler "github.com/nimbus/api/handlers/file"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFileHandlerDB(t *testing.T) *gorm.DB {
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

func createFileHandlerUser(t *testing.T, db *gorm.DB) (*models.User, *models.Box) {
	t.Helper()
	boxID, _ := utils.GenerateSecureID()
	userID, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")
	u := &models.User{
		ID:      userID,
		Email:   fmt.Sprintf("filehandler-%d@example.com", userID),
		Password: hash,
		PassKey:  "1234",
		Boxes:   []models.Box{{Name: "Test-Box", BoxID: boxID}},
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return u, &u.Boxes[0]
}

func fileListRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/files", func(c *gin.Context) {
		filehandler.List(s3db.Config{}, db, c)
	})
	return r
}

func fileRenameRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/files/rename", func(c *gin.Context) {
		filehandler.Rename(s3db.Config{}, db, c)
	})
	return r
}

func fileMoveRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/files/move", func(c *gin.Context) {
		filehandler.Move(s3db.Config{}, db, c)
	})
	return r
}

// --- List ---

func TestListFiles_Unauthorized(t *testing.T) {
	db := setupFileHandlerDB(t)
	r := fileListRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/files?box_name=Test-Box", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListFiles_MissingBoxName(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, _ := createFileHandlerUser(t, db)
	r := fileListRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/files", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "box_name is required", body["error"])
}

func TestListFiles_BoxNotOwned(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, _ := createFileHandlerUser(t, db)
	r := fileListRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/files?box_name=Other-Box", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestListFiles_EmptyBox(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, _ := createFileHandlerUser(t, db)
	r := fileListRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/files?box_name=Test-Box", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	files := body["files"].([]interface{})
	assert.Equal(t, 0, len(files))
}

func TestListFiles_WithFiles(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, b := createFileHandlerUser(t, db)
	r := fileListRouter(db)

	db.Create(&models.File{UserID: u.ID, BoxID: b.ID, Name: "a.txt", Size: 100, S3Key: "list-a.txt", Confirmed: true})
	db.Create(&models.File{UserID: u.ID, BoxID: b.ID, Name: "b.txt", Size: 200, S3Key: "list-b.txt", Confirmed: true})

	req, _ := http.NewRequest(http.MethodGet, "/files?box_name=Test-Box", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	files := body["files"].([]interface{})
	assert.Equal(t, 2, len(files))
}

func TestListFiles_OnlyShowsOwnFiles(t *testing.T) {
	db := setupFileHandlerDB(t)
	u1, b1 := createFileHandlerUser(t, db)

	// Second user with their own files
	boxID2, _ := utils.GenerateSecureID()
	userID2, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")
	u2 := &models.User{
		ID: userID2, Email: fmt.Sprintf("other-%d@example.com", userID2),
		Password: hash, PassKey: "1234",
		Boxes: []models.Box{{Name: "Test-Box", BoxID: boxID2}},
	}
	db.Create(u2)

	db.Create(&models.File{UserID: u1.ID, BoxID: b1.ID, Name: "mine.txt", Size: 100, S3Key: "mine-key.txt", Confirmed: true})
	db.Create(&models.File{UserID: u2.ID, BoxID: u2.Boxes[0].ID, Name: "theirs.txt", Size: 200, S3Key: "theirs-key.txt", Confirmed: true})

	r := fileListRouter(db)
	req, _ := http.NewRequest(http.MethodGet, "/files?box_name=Test-Box", nil)
	req.Header.Set("Authorization", authHeader(t, u1))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	files := body["files"].([]interface{})
	assert.Equal(t, 1, len(files), "should only see own files")
}

// --- Rename ---

func TestRenameFile_Unauthorized(t *testing.T) {
	db := setupFileHandlerDB(t)
	r := fileRenameRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/files/rename?box_name=Test-Box&key=old.txt&new_name=new.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRenameFile_MissingParams(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, _ := createFileHandlerUser(t, db)
	r := fileRenameRouter(db)

	cases := []string{
		"/files/rename?key=k.txt&new_name=n.txt",         // missing box_name
		"/files/rename?box_name=Test-Box&new_name=n.txt", // missing key
		"/files/rename?box_name=Test-Box&key=k.txt",      // missing new_name
	}
	for _, path := range cases {
		req, _ := http.NewRequest(http.MethodPatch, path, nil)
		req.Header.Set("Authorization", authHeader(t, u))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "path: %s", path)
	}
}

func TestRenameFile_BoxNotOwned(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, _ := createFileHandlerUser(t, db)
	r := fileRenameRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/files/rename?box_name=NotMyBox&key=k.txt&new_name=n.txt", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRenameFile_FileNotFound(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, _ := createFileHandlerUser(t, db)
	r := fileRenameRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/files/rename?box_name=Test-Box&key=ghost.txt&new_name=n.txt", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "file not found", body["error"])
}

func TestRenameFile_Success(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, b := createFileHandlerUser(t, db)
	r := fileRenameRouter(db)

	f := models.File{UserID: u.ID, BoxID: b.ID, Name: "original.txt", Size: 100, S3Key: "rename-test-key.txt"}
	db.Create(&f)

	req, _ := http.NewRequest(http.MethodPatch, "/files/rename?box_name=Test-Box&key=rename-test-key.txt&new_name=renamed.txt", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "file renamed", body["message"])
	assert.Equal(t, "renamed.txt", body["name"])

	var updated models.File
	db.First(&updated, f.ID)
	assert.Equal(t, "renamed.txt", updated.Name)
}

// --- Move ---

func TestMoveFile_Unauthorized(t *testing.T) {
	db := setupFileHandlerDB(t)
	r := fileMoveRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/files/move?box_name=Test-Box&key=f.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMoveFile_MissingParams(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, _ := createFileHandlerUser(t, db)
	r := fileMoveRouter(db)

	cases := []string{
		"/files/move?key=f.txt",          // missing box_name
		"/files/move?box_name=Test-Box",  // missing key
	}
	for _, path := range cases {
		req, _ := http.NewRequest(http.MethodPatch, path, nil)
		req.Header.Set("Authorization", authHeader(t, u))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "path: %s", path)
	}
}

func TestMoveFile_BoxNotOwned(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, _ := createFileHandlerUser(t, db)
	r := fileMoveRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/files/move?box_name=NotMine&key=f.txt", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestMoveFile_FileNotFound(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, _ := createFileHandlerUser(t, db)
	r := fileMoveRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/files/move?box_name=Test-Box&key=ghost.txt", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMoveFile_ToRootSuccess(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, b := createFileHandlerUser(t, db)

	folder := models.Folder{Name: "docs", UserID: u.ID, BoxID: b.ID}
	db.Create(&folder)

	f := models.File{UserID: u.ID, BoxID: b.ID, FolderID: &folder.ID, Name: "f.txt", Size: 100, S3Key: "move-root-key.txt"}
	db.Create(&f)

	r := fileMoveRouter(db)
	// No target_path = move to root
	req, _ := http.NewRequest(http.MethodPatch, "/files/move?box_name=Test-Box&key=move-root-key.txt", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "file moved", body["message"])

	var updated models.File
	db.First(&updated, f.ID)
	assert.Nil(t, updated.FolderID, "file should be at root level")
}

func TestMoveFile_ToFolderSuccess(t *testing.T) {
	db := setupFileHandlerDB(t)
	u, b := createFileHandlerUser(t, db)

	dest := models.Folder{Name: "archive", UserID: u.ID, BoxID: b.ID}
	db.Create(&dest)

	f := models.File{UserID: u.ID, BoxID: b.ID, Name: "report.txt", Size: 500, S3Key: "move-folder-key.txt"}
	db.Create(&f)

	r := fileMoveRouter(db)
	req, _ := http.NewRequest(http.MethodPatch, "/files/move?box_name=Test-Box&key=move-folder-key.txt&target_path=archive", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updated models.File
	db.First(&updated, f.ID)
	assert.NotNil(t, updated.FolderID)
	assert.Equal(t, dest.ID, *updated.FolderID)
}
