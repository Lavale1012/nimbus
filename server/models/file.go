package models

import "gorm.io/gorm"

type File struct {
	gorm.Model
	Name     string  `gorm:"not null" json:"name"`
	Size     int64   `gorm:"default:0" json:"size"`
	S3Key    string  `gorm:"unique;not null" json:"s3_key"`
	UserID   uint    `gorm:"not null;index" json:"user_id"`
	BoxID    uint    `gorm:"not null;index" json:"box_id"`
	FolderID *uint   `gorm:"index" json:"folder_id"` // NEW
	User     User    `gorm:"constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Box      Box     `gorm:"constraint:OnDelete:CASCADE" json:"box,omitempty"`
	Folder   *Folder `gorm:"constraint:OnDelete:SET NULL" json:"folder,omitempty"` // NEW
}
