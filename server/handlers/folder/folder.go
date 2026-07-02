package folder

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	s3db "github.com/nimbus/api/db/s3"
	"github.com/nimbus/api/middleware/jwt"
	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils/helpers"
	"gorm.io/gorm"
)

// ListResponse represents the contents of a folder
type ListResponse struct {
	FolderID   *uint         `json:"folder_id"`
	FolderName string        `json:"folder_name"`
	Path       string        `json:"path"`
	Files      []FileEntry   `json:"files"`
	Folders    []FolderEntry `json:"folders"`
}

type FileEntry struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	S3Key     string `json:"s3_key"`
	CreatedAt string `json:"created_at"`
}

type FolderEntry struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func Create(h s3db.Config, c *gin.Context, db *gorm.DB) {
	var err error
	var user *models.User
	user, err = jwt.AuthenticateUser(c, db)
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	const MAX_FOLDER_NAME_LENGTH int = 25

	if h.Bucket == "" || h.Client == nil {
		c.JSON(500, gin.H{"error": "S3 client or bucket not configured"})
		return
	}

	boxName := c.Query("box_name")
	if boxName == "" {
		c.JSON(400, gin.H{"error": "box name is required"})
		return
	}

	foldername := c.Query("folder_name")
	if foldername == "" {
		c.JSON(400, gin.H{"error": "folder name is required"})
		return
	}

	if len(foldername) > MAX_FOLDER_NAME_LENGTH {
		c.JSON(400, gin.H{"error": fmt.Sprintf("folder name must be at most %d characters", MAX_FOLDER_NAME_LENGTH)})
		return
	}

	_, err = helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		c.JSON(403, gin.H{"error": "box not found or access denied"})
		return
	}

	// Sanitize the folder name
	sanitizedName := filepath.Base(foldername)
	sanitizedName = strings.ReplaceAll(sanitizedName, " ", "_")

	Path := c.Query("path")

	// Build the S3 key with trailing slash to represent a folder
	var key string

	if Path == "" {
		key = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/", user.ID, boxName, sanitizedName)
	} else {
		key = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/%s/", user.ID, boxName, Path, sanitizedName)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	_, err = h.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &h.Bucket,
		Key:    &key,
		Body:   strings.NewReader(""),
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create folder"})
		return
	} else {
		err = db.Create(&models.Folder{
			Name:     sanitizedName,
			UserID:   user.ID,
			BoxID:    helpers.GetBoxID(db, boxName, user.ID),
			ParentID: helpers.GetParentFolderID(db, user.ID, boxName, Path),
		}).Error
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to create folder in database"})
			return
		}
	}

	c.JSON(200, gin.H{"message": "Folder created successfully", "folder": key})
}

func Download(h s3db.Config, c *gin.Context, db *gorm.DB) {
	var err error
	var user *models.User
	user, err = jwt.AuthenticateUser(c, db)
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	if h.Bucket == "" || h.Client == nil {
		c.JSON(500, gin.H{"error": "S3 client or bucket not configured"})
		return
	}

	boxName := c.Query("box_name")
	if boxName == "" {
		c.JSON(400, gin.H{"error": "box name is required"})
		return
	}

	foldername := c.Query("folder_name")
	if foldername == "" {
		c.JSON(400, gin.H{"error": "folder name is required"})
		return
	}

	_, err = helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		c.JSON(403, gin.H{"error": "box not found or access denied"})
		return
	}

	// Sanitize the folder name
	sanitizedName := filepath.Base(foldername)
	sanitizedName = strings.ReplaceAll(sanitizedName, " ", "_")

	Path := c.Query("path")

	var key string

	if Path == "" {
		key = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/", user.ID, boxName, sanitizedName)
	} else {
		key = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/%s/", user.ID, boxName, Path, sanitizedName)
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Collect all object keys, paginating through results (S3 returns max 1,000 per call).
	var allKeys []string
	var continuationToken *string
	for {
		page, err := h.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            &h.Bucket,
			Prefix:            &key,
			ContinuationToken: continuationToken,
		})
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to list folder contents"})
			return
		}
		for _, obj := range page.Contents {
			allKeys = append(allKeys, *obj.Key)
		}
		if page.IsTruncated == nil || !*page.IsTruncated || page.NextContinuationToken == nil {
			break
		}
		continuationToken = page.NextContinuationToken
	}

	if len(allKeys) == 0 {
		c.JSON(404, gin.H{"error": "folder is empty or not found"})
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, sanitizedName))

	zipWriter := zip.NewWriter(c.Writer)
	// Streamed straight to the HTTP response; headers are already sent, so a
	// close error here isn't recoverable — ignore it explicitly.
	defer func() { _ = zipWriter.Close() }()

	for _, objKey := range allKeys {
		Output, err := h.Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: &h.Bucket,
			Key:    &objKey,
		})
		if err != nil {
			continue
		}

		Name := strings.TrimPrefix(objKey, key)
		if Name == "" {
			_ = Output.Body.Close()
			continue
		}

		w, err := zipWriter.Create(Name)
		if err != nil {
			_ = Output.Body.Close()
			continue
		}

		_, err = io.Copy(w, Output.Body)
		if err != nil {
			_ = Output.Body.Close()
			continue
		}
		_ = Output.Body.Close()
	}
}

func List(h s3db.Config, c *gin.Context, db *gorm.DB) {
	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	boxName := c.Query("box_name")
	if boxName == "" {
		c.JSON(400, gin.H{"error": "box_name is required"})
		return
	}

	box, err := helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		c.JSON(403, gin.H{"error": "box not found or access denied"})
		return
	}

	pathParam := c.Query("path")

	var folderID *uint
	var folderName string
	var path string

	if pathParam != "" && pathParam != "/" {
		pathParam = strings.Trim(pathParam, "/")
		segments := strings.Split(pathParam, "/")

		var currentParentID *uint = nil

		for i, segment := range segments {
			var folder models.Folder
			query := db.Where("name = ? AND user_id = ? AND box_id = ?", segment, user.ID, box.ID)

			if currentParentID == nil {
				query = query.Where("parent_id IS NULL")
			} else {
				query = query.Where("parent_id = ?", *currentParentID)
			}

			if err := query.First(&folder).Error; err != nil {
				c.JSON(404, gin.H{"error": fmt.Sprintf("folder not found: %s", segment)})
				return
			}

			if i == len(segments)-1 {
				folderID = &folder.ID
				folderName = folder.Name
				path = "/" + pathParam
			} else {
				currentParentID = &folder.ID
			}
		}
	} else {
		folderName = boxName
		path = "/"
	}

	var files []models.File
	if folderID == nil {
		db.Where("user_id = ? AND box_id = ? AND folder_id IS NULL", user.ID, box.ID).Find(&files)
	} else {
		db.Where("user_id = ? AND box_id = ? AND folder_id = ?", user.ID, box.ID, *folderID).Find(&files)
	}

	var subfolders []models.Folder
	if folderID == nil {
		db.Where("user_id = ? AND box_id = ? AND parent_id IS NULL", user.ID, box.ID).Find(&subfolders)
	} else {
		db.Where("user_id = ? AND box_id = ? AND parent_id = ?", user.ID, box.ID, *folderID).Find(&subfolders)
	}

	fileEntries := make([]FileEntry, len(files))
	for i, f := range files {
		fileEntries[i] = FileEntry{
			ID:        f.ID,
			Name:      f.Name,
			Size:      f.Size,
			S3Key:     f.S3Key,
			CreatedAt: f.CreatedAt.Format(time.RFC3339),
		}
	}

	folderEntries := make([]FolderEntry, len(subfolders))
	for i, f := range subfolders {
		folderEntries[i] = FolderEntry{
			ID:        f.ID,
			Name:      f.Name,
			CreatedAt: f.CreatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(200, ListResponse{
		FolderID:   folderID,
		FolderName: folderName,
		Path:       path,
		Files:      fileEntries,
		Folders:    folderEntries,
	})
}
func Move(h s3db.Config, c *gin.Context, db *gorm.DB) {
	c.JSON(501, gin.H{"error": "folder move is not yet implemented"})
}

func Upload(h s3db.Config, c *gin.Context) {
	c.JSON(501, gin.H{"error": "folder upload is not yet implemented"})
}

func Rename(h s3db.Config, c *gin.Context, db *gorm.DB) {
	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	boxName := c.Query("box_name")
	if boxName == "" {
		c.JSON(400, gin.H{"error": "box_name is required"})
		return
	}

	folderName := c.Query("folder_name")
	if folderName == "" {
		c.JSON(400, gin.H{"error": "folder_name is required"})
		return
	}

	newName := c.Query("new_name")
	if newName == "" {
		c.JSON(400, gin.H{"error": "new_name is required"})
		return
	}

	box, err := helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		c.JSON(403, gin.H{"error": "box not found or access denied"})
		return
	}

	pathParam := strings.Trim(c.Query("path"), "/")

	// Resolve the folder in the DB
	folderID := helpers.GetParentFolderID(db, user.ID, boxName, func() string {
		if pathParam == "" {
			return folderName
		}
		return pathParam + "/" + folderName
	}())
	if folderID == nil {
		c.JSON(404, gin.H{"error": "folder not found"})
		return
	}

	// Check new name isn't already taken under the same parent
	var existing models.Folder
	parentID := helpers.GetParentFolderID(db, user.ID, boxName, pathParam)
	q := db.Where("name = ? AND user_id = ? AND box_id = ?", newName, user.ID, box.ID)
	if parentID == nil {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", *parentID)
	}
	if q.First(&existing).Error == nil {
		c.JSON(409, gin.H{"error": "a folder with that name already exists"})
		return
	}

	sanitizedNew := filepath.Base(newName)
	sanitizedNew = strings.ReplaceAll(sanitizedNew, " ", "_")

	// Rename in S3: copy all objects under old prefix to new prefix, then delete old
	if h.Client != nil && h.Bucket != "" {
		var oldPrefix, newPrefix string
		if pathParam == "" {
			oldPrefix = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/", user.ID, box.Name, folderName)
			newPrefix = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/", user.ID, box.Name, sanitizedNew)
		} else {
			oldPrefix = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/%s/", user.ID, box.Name, pathParam, folderName)
			newPrefix = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/%s/", user.ID, box.Name, pathParam, sanitizedNew)
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()

		// Paginate through all objects under the old prefix.
		var oldKeys []string
		var ct *string
		for {
			page, err := h.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
				Bucket:            &h.Bucket,
				Prefix:            &oldPrefix,
				ContinuationToken: ct,
			})
			if err != nil {
				c.JSON(500, gin.H{"error": "failed to list folder contents in S3"})
				return
			}
			for _, obj := range page.Contents {
				oldKeys = append(oldKeys, *obj.Key)
			}
			if page.IsTruncated == nil || !*page.IsTruncated || page.NextContinuationToken == nil {
				break
			}
			ct = page.NextContinuationToken
		}

		// Copy each object to the new prefix, tracking what was successfully copied
		// so we can roll back if a subsequent delete fails.
		var copiedNewKeys []string
		for _, oldKey := range oldKeys {
			newKey := newPrefix + strings.TrimPrefix(oldKey, oldPrefix)
			copySource := h.Bucket + "/" + oldKey

			if _, err := h.Client.CopyObject(ctx, &s3.CopyObjectInput{
				Bucket:     &h.Bucket,
				CopySource: &copySource,
				Key:        &newKey,
			}); err != nil {
				// Roll back any copies already written. Best-effort: a failed
				// rollback delete leaves an orphaned object but shouldn't mask the
				// original error, so the delete error is intentionally ignored.
				for _, k := range copiedNewKeys {
					_, _ = h.Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &h.Bucket, Key: &k})
				}
				c.JSON(500, gin.H{"error": "failed to copy S3 objects during rename"})
				return
			}
			copiedNewKeys = append(copiedNewKeys, newKey)
		}

		// All copies succeeded — delete the originals.
		for _, oldKey := range oldKeys {
			if _, err := h.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: &h.Bucket,
				Key:    &oldKey,
			}); err != nil {
				// Non-fatal: the new keys already exist and the DB will be updated.
				// Log and continue so one stale original doesn't abort the rename.
				fmt.Printf("[RENAME] warning: failed to delete old S3 key %s: %v\n", oldKey, err)
			}
		}
	}

	// Rename in DB
	if err := db.Model(&models.Folder{}).Where("id = ?", *folderID).Update("name", sanitizedNew).Error; err != nil {
		c.JSON(500, gin.H{"error": "failed to rename folder in database"})
		return
	}

	c.JSON(200, gin.H{"message": "folder renamed successfully", "new_name": sanitizedNew})
}

func Delete(h s3db.Config, c *gin.Context, db *gorm.DB) {
	user, err := jwt.AuthenticateUser(c, db)
	if err != nil {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	boxName := c.Query("box_name")
	if boxName == "" {
		c.JSON(400, gin.H{"error": "box_name is required"})
		return
	}

	folderName := c.Query("folder_name")
	if folderName == "" {
		c.JSON(400, gin.H{"error": "folder_name is required"})
		return
	}

	box, err := helpers.ValidateBoxOwnership(db, boxName, user.ID)
	if err != nil {
		c.JSON(403, gin.H{"error": "box not found or access denied"})
		return
	}

	pathParam := strings.Trim(c.Query("path"), "/")

	// Resolve the target folder in the DB
	folderID := helpers.GetParentFolderID(db, user.ID, boxName, func() string {
		if pathParam == "" {
			return folderName
		}
		return pathParam + "/" + folderName
	}())
	if folderID == nil {
		c.JSON(404, gin.H{"error": "folder not found"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Build the S3 prefix for this folder
	var s3Prefix string
	if pathParam == "" {
		s3Prefix = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/", user.ID, box.Name, folderName)
	} else {
		s3Prefix = fmt.Sprintf("users/nim-user-%d/boxes/%s/%s/%s/", user.ID, box.Name, pathParam, folderName)
	}

	// Delete all S3 objects under the prefix (skip if S3 not configured)
	if h.Client != nil && h.Bucket != "" {
		list, err := h.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: &h.Bucket,
			Prefix: &s3Prefix,
		})
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to list folder contents in S3"})
			return
		}
		for _, obj := range list.Contents {
			key := *obj.Key
			// Best-effort cascade delete; a failure here is logged upstream and
			// leaves an orphaned object rather than aborting the whole delete.
			_, _ = h.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: &h.Bucket,
				Key:    &key,
			})
		}
	}

	// Recursively delete all DB records under this folder (files + subfolders)
	if err := deleteFolderTree(db, *folderID); err != nil {
		c.JSON(500, gin.H{"error": "failed to delete folder records from database"})
		return
	}

	c.JSON(200, gin.H{"message": "folder deleted successfully"})
}

// deleteFolderTree recursively deletes a folder and all its contents from the DB.
func deleteFolderTree(db *gorm.DB, folderID uint) error {
	var subfolders []models.Folder
	db.Where("parent_id = ?", folderID).Find(&subfolders)
	for _, sub := range subfolders {
		if err := deleteFolderTree(db, sub.ID); err != nil {
			return err
		}
	}
	if err := db.Where("folder_id = ?", folderID).Delete(&models.File{}).Error; err != nil {
		return err
	}
	return db.Delete(&models.Folder{}, folderID).Error
}
