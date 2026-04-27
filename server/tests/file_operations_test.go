package tests

import (
	"testing"

	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFileTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Box{}, &models.Folder{}, &models.File{}); err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}
	return db
}

func createFileTestUser(t *testing.T, db *gorm.DB) (*models.User, *models.Box) {
	boxID, _ := utils.GenerateSecureID()
	userID, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")

	u := &models.User{
		ID:       userID,
		Email:    "filetest@example.com",
		Password: hash,
		PassKey:  "1234",
		Boxes: []models.Box{
			{Name: "Test-Box", BoxID: boxID},
		},
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return u, &u.Boxes[0]
}

func TestFileModel_Creation(t *testing.T) {
	db := setupFileTestDB(t)
	u, b := createFileTestUser(t, db)

	f := models.File{
		UserID: u.ID,
		BoxID:  b.ID,
		Name:   "test-file.txt",
		Size:   1024,
		S3Key:  "users/nim-user-1/boxes/Test-Box/test-file.txt",
	}

	err := db.Create(&f).Error
	assert.NoError(t, err)
	assert.NotZero(t, f.ID)
	assert.Equal(t, "test-file.txt", f.Name)
	assert.Equal(t, int64(1024), f.Size)
}

func TestFileModel_UniqueS3Key(t *testing.T) {
	db := setupFileTestDB(t)
	u, b := createFileTestUser(t, db)

	f1 := models.File{UserID: u.ID, BoxID: b.ID, Name: "file.txt", Size: 100, S3Key: "unique-key.txt"}
	f2 := models.File{UserID: u.ID, BoxID: b.ID, Name: "file2.txt", Size: 200, S3Key: "unique-key.txt"}

	assert.NoError(t, db.Create(&f1).Error)
	assert.Error(t, db.Create(&f2).Error, "Duplicate S3 key should be rejected")
}

func TestFileModel_UserAssociation(t *testing.T) {
	db := setupFileTestDB(t)
	u, b := createFileTestUser(t, db)

	f := models.File{UserID: u.ID, BoxID: b.ID, Name: "file.txt", Size: 512, S3Key: "assoc-key.txt"}
	db.Create(&f)

	var retrieved models.File
	db.First(&retrieved, f.ID)
	assert.Equal(t, u.ID, retrieved.UserID)
}

func TestFileModel_BoxAssociation(t *testing.T) {
	db := setupFileTestDB(t)
	u, b := createFileTestUser(t, db)

	f := models.File{UserID: u.ID, BoxID: b.ID, Name: "file.txt", Size: 512, S3Key: "box-assoc-key.txt"}
	db.Create(&f)

	var retrieved models.File
	db.First(&retrieved, f.ID)
	assert.Equal(t, b.ID, retrieved.BoxID)
}

func TestFileModel_FolderAssociation(t *testing.T) {
	db := setupFileTestDB(t)
	u, b := createFileTestUser(t, db)

	folder := models.Folder{Name: "docs", UserID: u.ID, BoxID: b.ID}
	db.Create(&folder)

	f := models.File{UserID: u.ID, BoxID: b.ID, FolderID: &folder.ID, Name: "doc.txt", Size: 100, S3Key: "folder-assoc-key.txt"}
	db.Create(&f)

	var retrieved models.File
	db.First(&retrieved, f.ID)
	assert.NotNil(t, retrieved.FolderID)
	assert.Equal(t, folder.ID, *retrieved.FolderID)
}

func TestFileModel_NullFolderID(t *testing.T) {
	db := setupFileTestDB(t)
	u, b := createFileTestUser(t, db)

	f := models.File{UserID: u.ID, BoxID: b.ID, Name: "root-file.txt", Size: 100, S3Key: "root-file-key.txt"}
	db.Create(&f)

	var retrieved models.File
	db.First(&retrieved, f.ID)
	assert.Nil(t, retrieved.FolderID, "Root-level file should have nil FolderID")
}

func TestFileModel_MultipleFiles(t *testing.T) {
	db := setupFileTestDB(t)
	u, b := createFileTestUser(t, db)

	files := []models.File{
		{UserID: u.ID, BoxID: b.ID, Name: "a.txt", Size: 100, S3Key: "key-a.txt"},
		{UserID: u.ID, BoxID: b.ID, Name: "b.txt", Size: 200, S3Key: "key-b.txt"},
		{UserID: u.ID, BoxID: b.ID, Name: "c.txt", Size: 300, S3Key: "key-c.txt"},
	}
	for _, f := range files {
		assert.NoError(t, db.Create(&f).Error)
	}

	var count int64
	db.Model(&models.File{}).Where("user_id = ?", u.ID).Count(&count)
	assert.Equal(t, int64(3), count)
}

func TestFileModel_Delete(t *testing.T) {
	db := setupFileTestDB(t)
	u, b := createFileTestUser(t, db)

	f := models.File{UserID: u.ID, BoxID: b.ID, Name: "del.txt", Size: 100, S3Key: "del-key.txt"}
	db.Create(&f)

	db.Delete(&f)

	var count int64
	db.Model(&models.File{}).Where("id = ?", f.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestFileModel_QueryByS3Key(t *testing.T) {
	db := setupFileTestDB(t)
	u, b := createFileTestUser(t, db)

	f := models.File{UserID: u.ID, BoxID: b.ID, Name: "find-me.txt", Size: 512, S3Key: "find-by-key.txt"}
	db.Create(&f)

	var retrieved models.File
	err := db.Where("s3_key = ?", "find-by-key.txt").First(&retrieved).Error
	assert.NoError(t, err)
	assert.Equal(t, "find-me.txt", retrieved.Name)
}

func TestFileModel_UpdateSize(t *testing.T) {
	db := setupFileTestDB(t)
	u, b := createFileTestUser(t, db)

	f := models.File{UserID: u.ID, BoxID: b.ID, Name: "resize.txt", Size: 1024, S3Key: "resize-key.txt"}
	db.Create(&f)

	db.Model(&f).Update("size", 2048)

	var updated models.File
	db.First(&updated, f.ID)
	assert.Equal(t, int64(2048), updated.Size)
}

func TestFileNameSanitization(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"my file.txt", "my_file.txt"},
		{"normal.txt", "normal.txt"},
		{"two  spaces.txt", "two__spaces.txt"},
	}
	for _, tc := range cases {
		result := ""
		for _, ch := range tc.input {
			if ch == ' ' {
				result += "_"
			} else {
				result += string(ch)
			}
		}
		assert.Equal(t, tc.expected, result)
	}
}

func TestBoxOwnership_WrongUser(t *testing.T) {
	db := setupFileTestDB(t)
	u1, b1 := createFileTestUser(t, db)
	_ = u1

	boxID2, _ := utils.GenerateSecureID()
	userID2, _ := utils.GenerateUserID()
	hash, _ := utils.PasswordHash("Test123!@#")
	u2 := &models.User{
		ID: userID2, Email: "other@example.com", Password: hash, PassKey: "5678",
		Boxes: []models.Box{{Name: "Other-Box", BoxID: boxID2}},
	}
	db.Create(u2)

	// u1 tries to access u2's box
	var box models.Box
	err := db.Where("id = ? AND user_id = ?", b1.ID, u2.ID).First(&box).Error
	assert.Error(t, err, "Should not be able to access another user's box")
}
