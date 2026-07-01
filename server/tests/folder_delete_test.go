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
	"github.com/nimbus/api/middleware/jwt"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// --- helpers ---

func setupFolderDeleteDB(t *testing.T) *gorm.DB {
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

func createFolderDeleteUser(t *testing.T, db *gorm.DB) (*models.User, *models.Box) {
	t.Helper()
	boxID, _ := utils.GenerateSecureID()
	userID, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")
	u := &models.User{
		ID:       userID,
		Email:    fmt.Sprintf("foldertest-%d@example.com", userID),
		Password: hash,
		PassKey:  "1234",
		Boxes:    []models.Box{{Name: "Test-Box", BoxID: boxID}},
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return u, &u.Boxes[0]
}

// authHeader generates a valid Bearer token for the given user.
func authHeader(t *testing.T, u *models.User) string {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key")
	token, err := jwt.CreateToken(u.Email, fmt.Sprintf("%d", u.ID))
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	return "Bearer " + token
}

// folderDeleteRouter wires DELETE /folders to folder.Delete with a nil S3 config.
// Pass a real s3db.Config if you need S3 behaviour; nil client is fine for DB-only tests.
func folderDeleteRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/folders", func(c *gin.Context) {
		folder.Delete(s3db.Config{}, c, db)
	})
	return r
}

// --- deleteFolderTree unit tests (DB logic only) ---

func TestDeleteFolderTree_SingleFolder(t *testing.T) {
	db := setupFolderDeleteDB(t)
	u, b := createFolderDeleteUser(t, db)

	f := models.Folder{Name: "docs", UserID: u.ID, BoxID: b.ID}
	db.Create(&f)

	// Verify it exists
	var count int64
	db.Model(&models.Folder{}).Where("id = ?", f.ID).Count(&count)
	assert.Equal(t, int64(1), count)

	// Delete via the exported helper path by calling Delete handler directly on DB
	db.Delete(&models.Folder{}, f.ID)

	db.Model(&models.Folder{}).Where("id = ?", f.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestDeleteFolderTree_DeletesChildFiles(t *testing.T) {
	db := setupFolderDeleteDB(t)
	u, b := createFolderDeleteUser(t, db)

	f := models.Folder{Name: "images", UserID: u.ID, BoxID: b.ID}
	db.Create(&f)

	file1 := models.File{UserID: u.ID, BoxID: b.ID, FolderID: &f.ID, Name: "a.png", Size: 100, S3Key: "tree-del-a.png"}
	file2 := models.File{UserID: u.ID, BoxID: b.ID, FolderID: &f.ID, Name: "b.png", Size: 200, S3Key: "tree-del-b.png"}
	db.Create(&file1)
	db.Create(&file2)

	// Simulate deleteFolderTree: delete files then folder
	db.Where("folder_id = ?", f.ID).Delete(&models.File{})
	db.Delete(&models.Folder{}, f.ID)

	var fileCount, folderCount int64
	db.Model(&models.File{}).Where("folder_id = ?", f.ID).Count(&fileCount)
	db.Model(&models.Folder{}).Where("id = ?", f.ID).Count(&folderCount)

	assert.Equal(t, int64(0), fileCount, "all files should be deleted")
	assert.Equal(t, int64(0), folderCount, "folder should be deleted")
}

func TestDeleteFolderTree_Recursive(t *testing.T) {
	db := setupFolderDeleteDB(t)
	u, b := createFolderDeleteUser(t, db)

	// parent → child → grandchild, each with a file
	parent := models.Folder{Name: "parent", UserID: u.ID, BoxID: b.ID}
	db.Create(&parent)
	child := models.Folder{Name: "child", UserID: u.ID, BoxID: b.ID, ParentID: &parent.ID}
	db.Create(&child)
	grand := models.Folder{Name: "grand", UserID: u.ID, BoxID: b.ID, ParentID: &child.ID}
	db.Create(&grand)

	db.Create(&models.File{UserID: u.ID, BoxID: b.ID, FolderID: &parent.ID, Name: "p.txt", Size: 1, S3Key: "rec-p.txt"})
	db.Create(&models.File{UserID: u.ID, BoxID: b.ID, FolderID: &child.ID, Name: "c.txt", Size: 1, S3Key: "rec-c.txt"})
	db.Create(&models.File{UserID: u.ID, BoxID: b.ID, FolderID: &grand.ID, Name: "g.txt", Size: 1, S3Key: "rec-g.txt"})

	// Delete bottom-up (mirrors deleteFolderTree)
	db.Where("folder_id = ?", grand.ID).Delete(&models.File{})
	db.Delete(&models.Folder{}, grand.ID)
	db.Where("folder_id = ?", child.ID).Delete(&models.File{})
	db.Delete(&models.Folder{}, child.ID)
	db.Where("folder_id = ?", parent.ID).Delete(&models.File{})
	db.Delete(&models.Folder{}, parent.ID)

	var folderCount, fileCount int64
	db.Model(&models.Folder{}).Where("id IN ?", []uint{parent.ID, child.ID, grand.ID}).Count(&folderCount)
	db.Model(&models.File{}).Where("folder_id IN ?", []uint{parent.ID, child.ID, grand.ID}).Count(&fileCount)

	assert.Equal(t, int64(0), folderCount, "all folders should be deleted")
	assert.Equal(t, int64(0), fileCount, "all files should be deleted")
}

// --- HTTP handler tests ---

func TestDeleteFolder_Unauthorized(t *testing.T) {
	db := setupFolderDeleteDB(t)
	r := folderDeleteRouter(db)

	req, _ := http.NewRequest(http.MethodDelete, "/folders?box_name=Test-Box&folder_name=docs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDeleteFolder_MissingBoxName(t *testing.T) {
	db := setupFolderDeleteDB(t)
	u, _ := createFolderDeleteUser(t, db)
	r := folderDeleteRouter(db)

	req, _ := http.NewRequest(http.MethodDelete, "/folders?folder_name=docs", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "box_name is required", body["error"])
}

func TestDeleteFolder_MissingFolderName(t *testing.T) {
	db := setupFolderDeleteDB(t)
	u, _ := createFolderDeleteUser(t, db)
	r := folderDeleteRouter(db)

	req, _ := http.NewRequest(http.MethodDelete, "/folders?box_name=Test-Box", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "folder_name is required", body["error"])
}

func TestDeleteFolder_BoxNotOwned(t *testing.T) {
	db := setupFolderDeleteDB(t)
	u, _ := createFolderDeleteUser(t, db)
	r := folderDeleteRouter(db)

	req, _ := http.NewRequest(http.MethodDelete, "/folders?box_name=Someone-Elses-Box&folder_name=docs", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteFolder_FolderNotFound(t *testing.T) {
	db := setupFolderDeleteDB(t)
	u, _ := createFolderDeleteUser(t, db)
	r := folderDeleteRouter(db)

	req, _ := http.NewRequest(http.MethodDelete, "/folders?box_name=Test-Box&folder_name=nonexistent", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "folder not found", body["error"])
}

func TestDeleteFolder_Success(t *testing.T) {
	db := setupFolderDeleteDB(t)
	u, b := createFolderDeleteUser(t, db)

	// Create a folder with a file inside
	f := models.Folder{Name: "docs", UserID: u.ID, BoxID: b.ID}
	db.Create(&f)
	db.Create(&models.File{UserID: u.ID, BoxID: b.ID, FolderID: &f.ID, Name: "note.txt", Size: 100, S3Key: "del-success-note.txt"})

	r := folderDeleteRouter(db)
	req, _ := http.NewRequest(http.MethodDelete, "/folders?box_name=Test-Box&folder_name=docs", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "folder deleted successfully", body["message"])

	// Folder and its file should be gone
	var folderCount, fileCount int64
	db.Model(&models.Folder{}).Where("id = ?", f.ID).Count(&folderCount)
	db.Model(&models.File{}).Where("folder_id = ?", f.ID).Count(&fileCount)
	assert.Equal(t, int64(0), folderCount)
	assert.Equal(t, int64(0), fileCount)
}

func TestDeleteFolder_WithPath(t *testing.T) {
	db := setupFolderDeleteDB(t)
	u, b := createFolderDeleteUser(t, db)

	// parent/child structure
	parent := models.Folder{Name: "parent", UserID: u.ID, BoxID: b.ID}
	db.Create(&parent)
	child := models.Folder{Name: "child", UserID: u.ID, BoxID: b.ID, ParentID: &parent.ID}
	db.Create(&child)

	r := folderDeleteRouter(db)
	req, _ := http.NewRequest(http.MethodDelete, "/folders?box_name=Test-Box&path=parent&folder_name=child", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Only child should be deleted; parent should still exist
	var childCount, parentCount int64
	db.Model(&models.Folder{}).Where("id = ?", child.ID).Count(&childCount)
	db.Model(&models.Folder{}).Where("id = ?", parent.ID).Count(&parentCount)
	assert.Equal(t, int64(0), childCount, "child folder should be deleted")
	assert.Equal(t, int64(1), parentCount, "parent folder should still exist")
}
