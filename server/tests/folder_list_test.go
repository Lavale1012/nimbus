package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
	folderhandler "github.com/nimbus/api/handlers/folder"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFolderListDB(t *testing.T) *gorm.DB {
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

func createFolderListUser(t *testing.T, db *gorm.DB) (*models.User, *models.Box) {
	t.Helper()
	boxID, _ := utils.GenerateSecureID()
	userID, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")
	u := &models.User{
		ID:       userID,
		Email:    fmt.Sprintf("folderlist-%d@example.com", userID),
		Password: hash,
		PassKey:  "1234",
		Boxes:    []models.Box{{Name: "Test-Box", BoxID: boxID}},
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return u, &u.Boxes[0]
}

func folderListRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/folders", func(c *gin.Context) {
		folderhandler.List(s3db.Config{}, c, db)
	})
	return r
}

// --- Unauthorized ---

func TestFolderList_Unauthorized(t *testing.T) {
	db := setupFolderListDB(t)
	r := folderListRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/folders?box_name=Test-Box", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Missing / invalid params ---

func TestFolderList_MissingBoxName(t *testing.T) {
	db := setupFolderListDB(t)
	u, _ := createFolderListUser(t, db)
	r := folderListRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/folders", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "box_name is required", body["error"])
}

func TestFolderList_BoxNotOwned(t *testing.T) {
	db := setupFolderListDB(t)
	u, _ := createFolderListUser(t, db)
	r := folderListRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/folders?box_name=NotMyBox", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestFolderList_PathFolderNotFound(t *testing.T) {
	db := setupFolderListDB(t)
	u, _ := createFolderListUser(t, db)
	r := folderListRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/folders?box_name=Test-Box&path=nonexistent", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Contains(t, body["error"], "folder not found")
}

// --- Root listing ---

func TestFolderList_RootEmpty(t *testing.T) {
	db := setupFolderListDB(t)
	u, _ := createFolderListUser(t, db)
	r := folderListRouter(db)

	req, _ := http.NewRequest(http.MethodGet, "/folders?box_name=Test-Box", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "/", body["path"])
	assert.Equal(t, "Test-Box", body["folder_name"])
	assert.Equal(t, 0, len(body["files"].([]interface{})))
	assert.Equal(t, 0, len(body["folders"].([]interface{})))
}

func TestFolderList_RootWithFilesAndFolders(t *testing.T) {
	db := setupFolderListDB(t)
	u, b := createFolderListUser(t, db)
	r := folderListRouter(db)

	db.Create(&models.Folder{Name: "documents", UserID: u.ID, BoxID: b.ID})
	db.Create(&models.Folder{Name: "images", UserID: u.ID, BoxID: b.ID})
	db.Create(&models.File{Name: "readme.txt", UserID: u.ID, BoxID: b.ID, Size: 512, S3Key: "fl-root-readme.txt"})

	req, _ := http.NewRequest(http.MethodGet, "/folders?box_name=Test-Box", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, 1, len(body["files"].([]interface{})))
	assert.Equal(t, 2, len(body["folders"].([]interface{})))
}

func TestFolderList_RootExplicitSlash(t *testing.T) {
	db := setupFolderListDB(t)
	u, b := createFolderListUser(t, db)
	r := folderListRouter(db)

	db.Create(&models.File{Name: "note.txt", UserID: u.ID, BoxID: b.ID, Size: 100, S3Key: "fl-slash-note.txt"})

	// path=/ should behave the same as no path
	req, _ := http.NewRequest(http.MethodGet, "/folders?box_name=Test-Box&path=/", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "/", body["path"])
	assert.Equal(t, 1, len(body["files"].([]interface{})))
}

// --- Subfolder listing ---

func TestFolderList_Subfolder(t *testing.T) {
	db := setupFolderListDB(t)
	u, b := createFolderListUser(t, db)
	r := folderListRouter(db)

	parent := models.Folder{Name: "documents", UserID: u.ID, BoxID: b.ID}
	db.Create(&parent)

	// files and subfolder inside documents
	db.Create(&models.Folder{Name: "work", UserID: u.ID, BoxID: b.ID, ParentID: &parent.ID})
	db.Create(&models.File{Name: "cv.pdf", UserID: u.ID, BoxID: b.ID, FolderID: &parent.ID, Size: 1024, S3Key: "fl-sub-cv.pdf"})

	// file at root — should NOT appear in subfolder listing
	db.Create(&models.File{Name: "root.txt", UserID: u.ID, BoxID: b.ID, Size: 50, S3Key: "fl-sub-root.txt"})

	req, _ := http.NewRequest(http.MethodGet, "/folders?box_name=Test-Box&path=documents", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	assert.Equal(t, "/documents", body["path"])
	assert.Equal(t, "documents", body["folder_name"])

	files := body["files"].([]interface{})
	folders := body["folders"].([]interface{})
	assert.Equal(t, 1, len(files), "should show only files inside documents")
	assert.Equal(t, 1, len(folders), "should show subfolder work")

	file := files[0].(map[string]interface{})
	assert.Equal(t, "cv.pdf", file["name"])

	folder := folders[0].(map[string]interface{})
	assert.Equal(t, "work", folder["name"])
}

func TestFolderList_NestedPath(t *testing.T) {
	db := setupFolderListDB(t)
	u, b := createFolderListUser(t, db)
	r := folderListRouter(db)

	parent := models.Folder{Name: "documents", UserID: u.ID, BoxID: b.ID}
	db.Create(&parent)
	child := models.Folder{Name: "work", UserID: u.ID, BoxID: b.ID, ParentID: &parent.ID}
	db.Create(&child)

	db.Create(&models.File{Name: "report.docx", UserID: u.ID, BoxID: b.ID, FolderID: &child.ID, Size: 2048, S3Key: "fl-nested-report.docx"})

	req, _ := http.NewRequest(http.MethodGet, "/folders?box_name=Test-Box&path=documents/work", nil)
	req.Header.Set("Authorization", authHeader(t, u))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	assert.Equal(t, "/documents/work", body["path"])
	assert.Equal(t, "work", body["folder_name"])
	files := body["files"].([]interface{})
	assert.Equal(t, 1, len(files))
	file := files[0].(map[string]interface{})
	assert.Equal(t, "report.docx", file["name"])
}

// --- User isolation ---

func TestFolderList_OnlyShowsOwnContents(t *testing.T) {
	db := setupFolderListDB(t)
	u1, b1 := createFolderListUser(t, db)

	// Second user with identically named box
	boxID2, _ := utils.GenerateSecureID()
	userID2, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")
	u2 := &models.User{
		ID:      userID2,
		Email:   fmt.Sprintf("other-fl-%d@example.com", userID2),
		Password: hash,
		PassKey:  "1234",
		Boxes:   []models.Box{{Name: "Test-Box", BoxID: boxID2}},
	}
	db.Create(u2)

	// u1 has a file and folder; u2 also has a file and folder in their own box
	db.Create(&models.Folder{Name: "mine", UserID: u1.ID, BoxID: b1.ID})
	db.Create(&models.File{Name: "mine.txt", UserID: u1.ID, BoxID: b1.ID, Size: 100, S3Key: "fl-iso-mine.txt"})
	db.Create(&models.Folder{Name: "theirs", UserID: u2.ID, BoxID: u2.Boxes[0].ID})
	db.Create(&models.File{Name: "theirs.txt", UserID: u2.ID, BoxID: u2.Boxes[0].ID, Size: 200, S3Key: "fl-iso-theirs.txt"})

	r := folderListRouter(db)
	req, _ := http.NewRequest(http.MethodGet, "/folders?box_name=Test-Box", nil)
	req.Header.Set("Authorization", authHeader(t, u1))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	assert.Equal(t, 1, len(body["files"].([]interface{})), "should only see own files")
	assert.Equal(t, 1, len(body["folders"].([]interface{})), "should only see own folders")
}
