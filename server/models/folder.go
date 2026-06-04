package models

import "gorm.io/gorm"

// Folder represents a directory inside a box. Folders are stored in a
// self-referencing tree: ParentID points to the parent folder, or is NULL
// for root-level folders that sit directly inside the box.
// The same tree structure is mirrored in S3 via key prefixes ending in "/".
type Folder struct {
	gorm.Model                                                               // CreatedAt, UpdatedAt, DeletedAt
	ID         uint     `gorm:"primaryKey" json:"id"`
	Name       string   `gorm:"not null" json:"name"`
	UserID     uint     `gorm:"not null;index" json:"user_id"`
	BoxID      uint     `gorm:"not null;index" json:"box_id"`
	ParentID   *uint    `gorm:"index" json:"parent_id"`               // nil = root folder inside its box
	Files      []File   `gorm:"foreignKey:FolderID" json:"files"`
	SubFolders []Folder `gorm:"foreignKey:ParentID" json:"sub_folders"` // nested sub-directories
}
