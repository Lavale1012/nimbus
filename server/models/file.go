package models

import "gorm.io/gorm"

// File holds the metadata for a file stored in S3. The actual bytes live in
// S3 under S3Key; the database only stores the reference and metadata so the
// API can look up, list, rename, and move files without touching S3 directly.
// FolderID is nil when the file sits at the root of its box (no folder).
type File struct {
	gorm.Model                                                                       // CreatedAt, UpdatedAt, DeletedAt
	Name     string  `gorm:"not null" json:"name"`                                   // display name (can differ from S3 key)
	Size     int64   `gorm:"default:0" json:"size"`                                  // file size in bytes
	S3Key    string  `gorm:"unique;not null" json:"s3_key"`                          // full S3 object key
	UserID   uint    `gorm:"not null;index" json:"user_id"`
	BoxID    uint    `gorm:"not null;index" json:"box_id"`
	FolderID *uint   `gorm:"index" json:"folder_id"`                                 // nil = file is at box root
	User     User    `gorm:"constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Box      Box     `gorm:"constraint:OnDelete:CASCADE" json:"box,omitempty"`
	Folder   *Folder `gorm:"constraint:OnDelete:SET NULL" json:"folder,omitempty"`   // SET NULL so deleting a folder un-nests its files
}
