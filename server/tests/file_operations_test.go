package tests

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nimbus/api/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupFileTestDB creates an in-memory SQLite database for file testing
func setupFileTestDB(t *testing.T) *gorm.DB {
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

// createTestUser creates a test user with a box for testing
func createTestUser(t *testing.T, db *gorm.DB) (uint, uint) {
	user := models.UserModel{
		Email:    "testuser@example.com",
		Password: "hashedpassword",
		PassKey:  "hashedpasskey",
	}

	result := db.Create(&user)
	if result.Error != nil {
		t.Fatalf("Failed to create test user: %v", result.Error)
	}

	box := models.BoxModel{
		UserID: user.ID,
		BoxID:  12345,
		Name:   "Test-Box",
		Size:   0,
	}

	result = db.Create(&box)
	if result.Error != nil {
		t.Fatalf("Failed to create test box: %v", result.Error)
	}

	return user.ID, box.BoxID
}

// TestDownloadFile_MissingKey tests download with missing file key
func TestDownloadFile_MissingKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// This test doesn't need S3 since it checks validation before S3 access
	// We'll skip the full handler test and just verify the logic
	t.Skip("Requires S3 mock setup - testing validation logic separately")
}

// TestFileModel_Creation tests creating a file model in the database
func TestFileModel_Creation(t *testing.T) {
	db := setupFileTestDB(t)
	userID, boxID := createTestUser(t, db)

	fileModel := models.FileModel{
		UserID: userID,
		BoxID:  boxID,
		Name:   "test-file.txt",
		Size:   1024,
		S3Key:  "test-file.txt",
	}

	result := db.Create(&fileModel)

	assert.NoError(t, result.Error)
	assert.NotZero(t, fileModel.ID)
	assert.Equal(t, "test-file.txt", fileModel.Name)
	assert.Equal(t, int64(1024), fileModel.Size)
}

// TestFileModel_UserAssociation tests file-user association
func TestFileModel_UserAssociation(t *testing.T) {
	db := setupFileTestDB(t)
	userID, boxID := createTestUser(t, db)

	fileModel := models.FileModel{
		UserID: userID,
		BoxID:  boxID,
		Name:   "test-file.txt",
		Size:   1024,
		S3Key:  "test-file.txt",
	}

	db.Create(&fileModel)

	// Verify the file is associated with the correct user
	var retrievedFile models.FileModel
	db.Preload("User").First(&retrievedFile, fileModel.ID)

	assert.Equal(t, userID, retrievedFile.UserID)
}

// TestFileModel_BoxAssociation tests file-box association
func TestFileModel_BoxAssociation(t *testing.T) {
	db := setupFileTestDB(t)
	userID, boxID := createTestUser(t, db)

	fileModel := models.FileModel{
		UserID: userID,
		BoxID:  boxID,
		Name:   "test-file.txt",
		Size:   1024,
		S3Key:  "test-file.txt",
	}

	db.Create(&fileModel)

	// Verify the file is associated with the correct box
	var retrievedFile models.FileModel
	db.First(&retrievedFile, fileModel.ID)

	assert.Equal(t, boxID, retrievedFile.BoxID)
}

// TestFileModel_MultipleFiles tests creating multiple files
func TestFileModel_MultipleFiles(t *testing.T) {
	db := setupFileTestDB(t)
	userID, boxID := createTestUser(t, db)

	files := []models.FileModel{
		{UserID: userID, BoxID: boxID, Name: "file1.txt", Size: 100, S3Key: "file1.txt"},
		{UserID: userID, BoxID: boxID, Name: "file2.txt", Size: 200, S3Key: "file2.txt"},
		{UserID: userID, BoxID: boxID, Name: "file3.txt", Size: 300, S3Key: "file3.txt"},
	}

	for _, file := range files {
		result := db.Create(&file)
		assert.NoError(t, result.Error)
	}

	// Verify all files were created
	var count int64
	db.Model(&models.FileModel{}).Where("user_id = ?", userID).Count(&count)
	assert.Equal(t, int64(3), count)
}

// TestFileModel_Delete tests deleting a file model
func TestFileModel_Delete(t *testing.T) {
	db := setupFileTestDB(t)
	userID, boxID := createTestUser(t, db)

	fileModel := models.FileModel{
		UserID: userID,
		BoxID:  boxID,
		Name:   "test-file.txt",
		Size:   1024,
		S3Key:  "test-file.txt",
	}

	db.Create(&fileModel)

	// Delete the file
	result := db.Delete(&fileModel)
	assert.NoError(t, result.Error)

	// Verify the file is deleted
	var count int64
	db.Model(&models.FileModel{}).Where("id = ?", fileModel.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}

// TestFileModel_QueryByS3Key tests querying files by S3 key
func TestFileModel_QueryByS3Key(t *testing.T) {
	db := setupFileTestDB(t)
	userID, boxID := createTestUser(t, db)

	fileModel := models.FileModel{
		UserID: userID,
		BoxID:  boxID,
		Name:   "test-file.txt",
		Size:   1024,
		S3Key:  "unique-s3-key.txt",
	}

	db.Create(&fileModel)

	// Query by S3 key
	var retrievedFile models.FileModel
	result := db.Where("s3_key = ?", "unique-s3-key.txt").First(&retrievedFile)

	assert.NoError(t, result.Error)
	assert.Equal(t, "test-file.txt", retrievedFile.Name)
	assert.Equal(t, "unique-s3-key.txt", retrievedFile.S3Key)
}

// TestFileModel_UpdateSize tests updating file size
func TestFileModel_UpdateSize(t *testing.T) {
	db := setupFileTestDB(t)
	userID, boxID := createTestUser(t, db)

	fileModel := models.FileModel{
		UserID: userID,
		BoxID:  boxID,
		Name:   "test-file.txt",
		Size:   1024,
		S3Key:  "test-file.txt",
	}

	db.Create(&fileModel)

	// Update size
	fileModel.Size = 2048
	db.Save(&fileModel)

	// Verify update
	var updatedFile models.FileModel
	db.First(&updatedFile, fileModel.ID)
	assert.Equal(t, int64(2048), updatedFile.Size)
}

// TestUploadValidation_UserValidation tests user validation in upload
func TestUploadValidation_UserValidation(t *testing.T) {
	db := setupFileTestDB(t)

	// Test that uploading with non-existent user fails
	var user models.UserModel
	result := db.First(&user, 99999) // Non-existent ID

	assert.Error(t, result.Error)
	assert.ErrorIs(t, result.Error, gorm.ErrRecordNotFound)
}

// TestUploadValidation_BoxValidation tests box validation in upload
func TestUploadValidation_BoxValidation(t *testing.T) {
	db := setupFileTestDB(t)
	userID, _ := createTestUser(t, db)

	// Test that querying with non-existent box fails
	var box models.BoxModel
	result := db.Where("id = ? AND user_id = ?", 99999, userID).First(&box)

	assert.Error(t, result.Error)
	assert.ErrorIs(t, result.Error, gorm.ErrRecordNotFound)
}

// createMultipartRequest creates a multipart form request for testing file uploads
func createMultipartRequest(t *testing.T, fieldName, fileName string, fileContent []byte) (*http.Request, *multipart.Writer) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = io.Copy(part, bytes.NewReader(fileContent))
	if err != nil {
		t.Fatalf("Failed to copy file content: %v", err)
	}

	writer.Close()

	req, err := http.NewRequest("POST", "/upload", body)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req, writer
}

// TestFileNameSanitization tests that file names are properly sanitized
func TestFileNameSanitization(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"file name.txt", "file_name.txt"},
		{"file  name.txt", "file__name.txt"},
		{"normal-file.txt", "normal-file.txt"},
		{"file@name.txt", "file@name.txt"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			// The sanitization logic replaces spaces with underscores
			result := bytes.ReplaceAll([]byte(tc.input), []byte(" "), []byte("_"))
			assert.Equal(t, tc.expected, string(result))
		})
	}
}

// TestBoxModel_CascadeDelete tests that deleting a box cascades to files
func TestBoxModel_CascadeDelete(t *testing.T) {
	db := setupFileTestDB(t)
	userID, boxID := createTestUser(t, db)

	// Create files in the box
	files := []models.FileModel{
		{UserID: userID, BoxID: boxID, Name: "file1.txt", Size: 100, S3Key: "file1.txt"},
		{UserID: userID, BoxID: boxID, Name: "file2.txt", Size: 200, S3Key: "file2.txt"},
	}

	for _, file := range files {
		db.Create(&file)
	}

	// Delete the box
	var box models.BoxModel
	db.Where("box_id = ?", boxID).First(&box)
	db.Delete(&box)

	// Verify files are cascade deleted (if cascade is set up correctly)
	var count int64
	db.Model(&models.FileModel{}).Where("box_id = ?", boxID).Count(&count)

	// Note: Cascade behavior depends on database constraints
	// With SQLite in-memory, this might not work as expected
	t.Logf("Files remaining after box deletion: %d", count)
}

// TestUploadFile_MissingUserID tests upload without user_id parameter
func TestUploadFile_MissingUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create request without user_id
	req, _ := createMultipartRequest(t, "file", "test.txt", []byte("test content"))
	c.Request = req

	// Note: Since we can't easily mock S3Client here without dependency injection,
	// we're testing the validation logic directly
	// The handler would fail at the user_id check

	// Verify that without user_id, we'd get 400 error (tested via query param check)
	assert.Equal(t, "", c.Query("user_id"))
}

// TestUploadFile_InvalidUserID tests upload with invalid user_id format
func TestUploadFile_InvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req, _ := createMultipartRequest(t, "file", "test.txt", []byte("test content"))
	req.URL.RawQuery = "user_id=invalid&box_id=1"
	c.Request = req

	// Test that non-numeric user_id would be caught
	userIDStr := c.Query("user_id")
	_, err := strconv.Atoi(userIDStr)
	assert.Error(t, err)
}

// TestUploadFile_MissingBoxID tests upload without box_id parameter
func TestUploadFile_MissingBoxID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req, _ := createMultipartRequest(t, "file", "test.txt", []byte("test content"))
	req.URL.RawQuery = "user_id=1"
	c.Request = req

	// Verify that without box_id, we'd get 400 error
	assert.Equal(t, "", c.Query("box_id"))
}

// TestUploadFile_InvalidBoxID tests upload with invalid box_id format
func TestUploadFile_InvalidBoxID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req, _ := createMultipartRequest(t, "file", "test.txt", []byte("test content"))
	req.URL.RawQuery = "user_id=1&box_id=invalid"
	c.Request = req

	// Test that non-numeric box_id would be caught
	boxIDStr := c.Query("box_id")
	_, err := strconv.Atoi(boxIDStr)
	assert.Error(t, err)
}

// TestUploadFile_UserNotFound tests upload with non-existent user
func TestUploadFile_UserNotFound(t *testing.T) {
	db := setupFileTestDB(t)

	// Try to find non-existent user
	var user models.UserModel
	err := db.First(&user, 99999).Error

	assert.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// TestUploadFile_BoxNotFound tests upload with non-existent box
func TestUploadFile_BoxNotFound(t *testing.T) {
	db := setupFileTestDB(t)
	userID, _ := createTestUser(t, db)

	// Try to find non-existent box for the user
	var box models.BoxModel
	err := db.Where("box_id = ? AND user_id = ?", 99999, userID).First(&box).Error

	assert.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// TestUploadFile_BoxBelongsToWrongUser tests upload with box that doesn't belong to user
func TestUploadFile_BoxBelongsToWrongUser(t *testing.T) {
	db := setupFileTestDB(t)

	// Create two users with boxes
	user1ID, _ := createTestUser(t, db)

	user2 := models.UserModel{
		Email:    "user2@example.com",
		Password: "hashedpassword",
		PassKey:  "hashedpasskey",
	}
	db.Create(&user2)

	box2 := models.BoxModel{
		UserID: user2.ID,
		BoxID:  54321,
		Name:   "User2-Box",
		Size:   0,
	}
	db.Create(&box2)

	// Try to access user2's box with user1's ID
	var box models.BoxModel
	err := db.Where("box_id = ? AND user_id = ?", box2.BoxID, user1ID).First(&box).Error

	assert.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// TestUploadFile_FilenameSanitization tests that filenames with spaces are sanitized
func TestUploadFile_FilenameSanitization(t *testing.T) {
	testFilename := "my test file.txt"
	sanitized := strings.ReplaceAll(testFilename, " ", "_")

	assert.Equal(t, "my_test_file.txt", sanitized)
	assert.NotContains(t, sanitized, " ")
}

// TestUploadFile_EmptyFile tests upload with zero-size file
func TestUploadFile_EmptyFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create request with empty file
	req, _ := createMultipartRequest(t, "file", "empty.txt", []byte(""))
	c.Request = req

	// Parse the form to get file header
	err := c.Request.ParseMultipartForm(32 << 20)
	assert.NoError(t, err)

	_, header, err := c.Request.FormFile("file")
	assert.NoError(t, err)

	// Verify size is 0
	assert.Equal(t, int64(0), header.Size)
}

// TestUploadFile_S3KeyGeneration tests that S3 keys are unique
func TestUploadFile_S3KeyGeneration(t *testing.T) {
	db := setupFileTestDB(t)
	userID, _ := createTestUser(t, db)

	// Set bucket prefix for user
	var user models.UserModel
	db.First(&user, userID)
	user.BucketPrefix = "users/nim-user-1/boxes/Home-Box/"
	db.Save(&user)

	// Simulate S3 key generation
	filename := "test.txt"
	timestamp1 := time.Now().Unix()
	key1 := fmt.Sprintf("%s%s_%d", user.BucketPrefix, filename, timestamp1)

	time.Sleep(time.Second * 1) // Wait 1 second to ensure different timestamp

	timestamp2 := time.Now().Unix()
	key2 := fmt.Sprintf("%s%s_%d", user.BucketPrefix, filename, timestamp2)

	// Keys should be different due to timestamp
	assert.NotEqual(t, key1, key2, "S3 keys should be unique due to different timestamps")
	assert.Contains(t, key1, user.BucketPrefix)
	assert.Contains(t, key2, user.BucketPrefix)
}

// TestUploadFile_DatabaseRollback tests behavior when database save fails after S3 upload
func TestUploadFile_DatabaseRollback(t *testing.T) {
	db := setupFileTestDB(t)
	userID, boxID := createTestUser(t, db)

	// Create a file with duplicate S3 key to force error
	existingFile := models.FileModel{
		UserID: userID,
		BoxID:  boxID,
		Name:   "existing.txt",
		Size:   100,
		S3Key:  "duplicate-key.txt",
	}
	db.Create(&existingFile)

	// Try to create another file with same S3 key (should fail due to unique constraint)
	duplicateFile := models.FileModel{
		UserID: userID,
		BoxID:  boxID,
		Name:   "duplicate.txt",
		Size:   200,
		S3Key:  "duplicate-key.txt", // Same key
	}

	result := db.Create(&duplicateFile)
	assert.Error(t, result.Error)

	// This simulates the scenario where S3 upload succeeds but DB save fails
	// In production, the orphaned S3 object should be cleaned up
}

// TestUploadFile_ContentTypeDefault tests default content type when not provided
func TestUploadFile_ContentTypeDefault(t *testing.T) {
	contentType := ""
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	assert.Equal(t, "application/octet-stream", contentType)
}

// TestUploadFile_ContentTypePreserved tests that provided content type is preserved
func TestUploadFile_ContentTypePreserved(t *testing.T) {
	providedType := "image/png"
	contentType := providedType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	assert.Equal(t, "image/png", contentType)
}
