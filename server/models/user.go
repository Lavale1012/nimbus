// Package models defines the GORM database models for Nimbus.
// Each struct maps to a table; GORM creates/updates the table via AutoMigrate.
package models

import (
	"gorm.io/gorm"
)

// User represents a registered Nimbus account.
// ID is set manually during registration (random 8-digit number) rather than
// relying on database auto-increment, which is why autoIncrement is false.
// Boxes are loaded eagerly when needed via GORM's Preload — cascading delete
// means removing a user automatically removes all their boxes.
type User struct {
	ID         uint   `gorm:"primaryKey;autoIncrement:false" json:"id"`
	Email      string `gorm:"unique;not null" json:"email"`
	Password   string `gorm:"not null" json:"-"` // stored as bcrypt hash
	PassKey    string `gorm:"not null" json:"-"` // stored as bcrypt hash
	Boxes      []Box  `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"boxes,omitempty"`
	gorm.Model        // adds CreatedAt, UpdatedAt, DeletedAt
}
