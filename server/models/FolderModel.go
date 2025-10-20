package models

import (
	"time"

	"gorm.io/gorm"
)

type FolderModel struct {
	gorm.Model
	ID         uint          `gorm:"primaryKey" json:"id"`
	Name       string        `gorm:"not null" json:"name"`
	UserID     string        `json:"user_id"`
	Box        string        `json:"box"`
	ParentID   *uint         `json:"parent_id"` // Pointer to allow null values for root folders
	User       UserModel     `gorm:"foreignKey:UserID;references:ID" json:"user"`
	BoxRef     BoxModel      `gorm:"foreignKey:Box;references:Name" json:"box_ref"`
	Parent     *FolderModel  `gorm:"foreignKey:ParentID;references:ID" json:"parent"`
	Children   []FolderModel `gorm:"foreignKey:ParentID;references:ID" json:"children"`
	Files      []FileModel   `gorm:"foreignKey:Folder;references:Name" json:"files"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}
