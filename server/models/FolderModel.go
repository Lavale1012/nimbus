package models

import "gorm.io/gorm"

type FolderModel struct {
	gorm.Model
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"not null" json:"name"`
	UserID   uint   `gorm:"not null;index" json:"user_id"`
	BoxID    uint   `gorm:"not null;index" json:"box_id"`
	ParentID *uint  `gorm:"index" json:"parent_id"` // Pointer to allow null values for root folders
}
