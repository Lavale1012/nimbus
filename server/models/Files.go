package models

import "gorm.io/gorm"

type FileModel struct {
	gorm.Model
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Box    string `json:"box"`
	Folder string `json:"folder"`
}
