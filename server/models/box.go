package models

import "gorm.io/gorm"

type Box struct {
	gorm.Model
	UserID  uint     `gorm:"not null;index" json:"user_id"`
	BoxID   uint     `gorm:"not null;index" json:"box_id"`
	Name    string   `gorm:"not null" json:"name"`
	Size    int64    `gorm:"default:0" json:"size"`
	Folders []Folder `gorm:"foreignKey:BoxID;constraint:OnDelete:CASCADE" json:"folders,omitempty"`
	Files   []File   `gorm:"foreignKey:BoxID;constraint:OnDelete:CASCADE" json:"files,omitempty"`
}
