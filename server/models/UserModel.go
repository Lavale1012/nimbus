package models

import "gorm.io/gorm"

type UserModel struct {
	gorm.Model
	ID       string     `json:"id"`
	Email    string     `json:"email"`
	Password string     `json:"password"`
	Boxes    []BoxModel `gorm:"foreignKey:UserID;references:ID" json:"boxes"`
}
