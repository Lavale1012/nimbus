package models

import "gorm.io/gorm"

type SectionModel struct {
	gorm.Model
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
}
