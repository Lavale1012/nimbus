package models

import "gorm.io/gorm"

type UserModel struct {
	gorm.Model
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
