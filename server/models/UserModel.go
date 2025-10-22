package models

import "gorm.io/gorm"

type UserModel struct {
	gorm.Model
	ID       uint       `gorm:"primaryKey" json:"id"`
	Email    string     `gorm:"unique;not null" json:"email"`
	Password string     `gorm:"not null" json:"password"`
	PassKey  string     `gorm:"not null" json:"passkey"`
	Boxes    []BoxModel `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"boxes,omitempty"`
}
