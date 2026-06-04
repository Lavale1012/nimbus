package models

import "gorm.io/gorm"

// Box is the top-level storage container for a user — think of it like a
// drive or a project root. Every user gets a "Home-Box" on registration and
// can create more. Boxes own Folders and Files; deleting a box cascades to
// everything inside it.
type Box struct {
	gorm.Model                                                             // CreatedAt, UpdatedAt, DeletedAt
	UserID  uint     `gorm:"not null;index" json:"user_id"`               // which user owns this box
	BoxID   uint     `gorm:"not null;index" json:"box_id"`                // cryptographically random ID (not the PK)
	Name    string   `gorm:"not null" json:"name"`                        // human-readable name, unique per user
	Size    int64    `gorm:"default:0" json:"size"`                       // total bytes stored (updated on upload/delete)
	Folders []Folder `gorm:"foreignKey:BoxID;constraint:OnDelete:CASCADE" json:"folders,omitempty"`
	Files   []File   `gorm:"foreignKey:BoxID;constraint:OnDelete:CASCADE" json:"files,omitempty"`
}
