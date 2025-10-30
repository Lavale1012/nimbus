package models

import (
	"gorm.io/gorm"
)

type UserModel struct {
	ID           uint       `gorm:"primaryKey;autoIncrement:false" json:"id"`
	Email        string     `gorm:"unique;not null" json:"email"`
	Password     string     `gorm:"not null" json:"password"`
	BucketPrefix string     `gorm:"unique" json:"bucket"`
	PassKey      string     `gorm:"not null" json:"passkey"`
	Boxes        []BoxModel `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"boxes,omitempty"`
	gorm.Model
}
