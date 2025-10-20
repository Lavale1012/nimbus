package models

import "gorm.io/gorm"

type BoxModel struct {
	gorm.Model
	UserID  uint          `gorm:"not null;index" json:"user_id"`
	Name    string        `gorm:"not null" json:"name"`
	Size    int64         `gorm:"default:0" json:"size"`
	User    UserModel     `gorm:"constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Folders []FolderModel `gorm:"foreignKey:BoxID;constraint:OnDelete:CASCADE" json:"folders,omitempty"`
	Files   []FileModel   `gorm:"foreignKey:BoxID;constraint:OnDelete:CASCADE" json:"files,omitempty"`
}
