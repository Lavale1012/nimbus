package models

import "gorm.io/gorm"

type Folder struct {
	gorm.Model
	ID         uint     `gorm:"primaryKey" json:"id"`
	Name       string   `gorm:"not null" json:"name"`
	UserID     uint     `gorm:"not null;index" json:"user_id"`
	BoxID      uint     `gorm:"not null;index" json:"box_id"`
	ParentID   *uint    `gorm:"index" json:"parent_id"` // Pointer to allow null values for root folders
	Files      []File   `gorm:"foreignKey:FolderID" json:"files"`
	SubFolders []Folder `gorm:"foreignKey:ParentID" json:"sub_folders"`
}
