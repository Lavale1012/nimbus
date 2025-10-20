package models

import "gorm.io/gorm"

type FolderModel struct {
	gorm.Model
	Name     string        `gorm:"not null" json:"name"`
	UserID   uint          `gorm:"not null;index" json:"user_id"`
	BoxID    uint          `gorm:"not null;index" json:"box_id"`
	ParentID *uint         `gorm:"index" json:"parent_id"` // Pointer to allow null values for root folders
	User     UserModel     `gorm:"constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Box      BoxModel      `gorm:"constraint:OnDelete:CASCADE" json:"box,omitempty"`
	Parent   *FolderModel  `gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE" json:"parent,omitempty"`
	Children []FolderModel `gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE" json:"children,omitempty"`
	Files    []FileModel   `gorm:"foreignKey:FolderID;constraint:OnDelete:CASCADE" json:"files,omitempty"`
}
