package models

import (
	"gorm.io/gorm"
)

type User struct {
	ID       uint   `gorm:"primaryKey;autoIncrement:false" json:"id"`
	Email    string `gorm:"unique;not null" json:"email"`
	Password string `gorm:"not null" json:"password"`
	PassKey  string `gorm:"not null" json:"passkey"`
	Boxes    []Box  `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"boxes,omitempty"`
	gorm.Model
}
