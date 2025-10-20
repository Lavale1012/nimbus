package models

import "gorm.io/gorm"

type BoxModel struct {
	gorm.Model
	ID      string        `json:"id"`
	UserID  string        `json:"user_id"`
	Name    string        `json:"name"`
	Size    int64         `json:"size"`
	User    UserModel     `gorm:"foreignKey:UserID;references:ID" json:"user"`
	Folders []FolderModel `gorm:"foreignKey:Box;references:Name" json:"folders"`
}
