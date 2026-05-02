package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
	"github.com/nimbus/api/handlers/folder"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFolderRenameDB(t *testing.T) *gorm.DB {
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

func createFolderRenameUser(t *testing.T, db *gorm.DB) (*models.User, *models.Box) {
	t.Helper()
	boxID, _ := utils.GenerateSecureID()
	userID, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")
	u := &models.User{
		ID:       userID,
		Email:    fmt.Sprintf("renametest-%d@example.com", userID),
		Password: hash,
		PassKey:  "1234",
		Boxes:    []models.Box{{Name: "Test-Box", BoxID: boxID}},
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return u, &u.Boxes[0]
}

func folderRenameRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/folders/rename", func(c *gin.Context) {
		folder.Rename(s3db.Config{}, c, db)
	})
	return r
}

func TestRenameFolder_Unauthorized(t *testing.T) {
	db := setupFolderRenameDB(t)
	r := folderRenameRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/folders/rename?box_name=Test-Box&folder_name=docs&new_name=notes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRenameFolder_MissingBoxName(t *testing.T) {
	db := setupFolderRenameDB(t)
	u, _ := createFolderRenameUser(t, db)
	r := folderRenameRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/folders/rename?folder_name=docs&new_name=notes", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "box_name is required", body["error"])
}

func TestRenameFolder_MissingFolderName(t *testing.T) {
	db := setupFolderRenameDB(t)
	u, _ := createFolderRenameUser(t, db)
	r := folderRenameRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/folders/rename?box_name=Test-Box&new_name=notes", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "folder_name is required", body["error"])
}

func TestRenameFolder_MissingNewName(t *testing.T) {
	db := setupFolderRenameDB(t)
	u, _ := createFolderRenameUser(t, db)
	r := folderRenameRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/folders/rename?box_name=Test-Box&folder_name=docs", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "new_name is required", body["error"])
}

func TestRenameFolder_BoxNotOwned(t *testing.T) {
	db := setupFolderRenameDB(t)
	u, _ := createFolderRenameUser(t, db)
	r := folderRenameRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/folders/rename?box_name=Not-My-Box&folder_name=docs&new_name=notes", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRenameFolder_FolderNotFound(t *testing.T) {
	db := setupFolderRenameDB(t)
	u, _ := createFolderRenameUser(t, db)
	r := folderRenameRouter(db)

	req, _ := http.NewRequest(http.MethodPatch, "/folders/rename?box_name=Test-Box&folder_name=nonexistent&new_name=notes", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "folder not found", body["error"])
}

func TestRenameFolder_ConflictWithExisting(t *testing.T) {
	db := setupFolderRenameDB(t)
	u, b := createFolderRenameUser(t, db)
	r := folderRenameRouter(db)

	db.Create(&models.Folder{Name: "docs", UserID: u.ID, BoxID: b.ID})
	db.Create(&models.Folder{Name: "notes", UserID: u.ID, BoxID: b.ID})

	req, _ := http.NewRequest(http.MethodPatch, "/folders/rename?box_name=Test-Box&folder_name=docs&new_name=notes", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "a folder with that name already exists", body["error"])
}

func TestRenameFolder_Success(t *testing.T) {
	db := setupFolderRenameDB(t)
	u, b := createFolderRenameUser(t, db)
	r := folderRenameRouter(db)

	f := models.Folder{Name: "docs", UserID: u.ID, BoxID: b.ID}
	db.Create(&f)

	req, _ := http.NewRequest(http.MethodPatch, "/folders/rename?box_name=Test-Box&folder_name=docs&new_name=notes", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "folder renamed successfully", body["message"])
	assert.Equal(t, "notes", body["new_name"])

	// Verify DB was updated
	var updated models.Folder
	db.First(&updated, f.ID)
	assert.Equal(t, "notes", updated.Name)
}

func TestRenameFolder_SpacesConvertedToUnderscores(t *testing.T) {
	db := setupFolderRenameDB(t)
	u, b := createFolderRenameUser(t, db)
	r := folderRenameRouter(db)

	db.Create(&models.Folder{Name: "docs", UserID: u.ID, BoxID: b.ID})

	req, _ := http.NewRequest(http.MethodPatch, "/folders/rename?box_name=Test-Box&folder_name=docs&new_name=my+notes", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "my_notes", body["new_name"])
}

func TestRenameFolder_WithPath(t *testing.T) {
	db := setupFolderRenameDB(t)
	u, b := createFolderRenameUser(t, db)
	r := folderRenameRouter(db)

	parent := models.Folder{Name: "parent", UserID: u.ID, BoxID: b.ID}
	db.Create(&parent)
	child := models.Folder{Name: "child", UserID: u.ID, BoxID: b.ID, ParentID: &parent.ID}
	db.Create(&child)

	req, _ := http.NewRequest(http.MethodPatch, "/folders/rename?box_name=Test-Box&path=parent&folder_name=child&new_name=renamed", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Child renamed, parent unchanged
	var updatedChild, updatedParent models.Folder
	db.First(&updatedChild, child.ID)
	db.First(&updatedParent, parent.ID)
	assert.Equal(t, "renamed", updatedChild.Name)
	assert.Equal(t, "parent", updatedParent.Name)
}
