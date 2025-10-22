package models

import "gorm.io/gorm"

type FileModel struct {
	gorm.Model
	Name    string         `gorm:"not null" json:"name"`
	Size    int64          `gorm:"default:0" json:"size"`
	S3Key   string         `gorm:"unique;not null" json:"s3_key"` // S3 object key for retrieval
	UserID  uint           `gorm:"not null;index" json:"user_id"`
	BoxID   uint           `gorm:"not null;index" json:"box_id"`
	User    UserModel      `gorm:"constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Box     BoxModel       `gorm:"constraint:OnDelete:CASCADE" json:"box,omitempty"`
	Folders []FolderModel  `gorm:"many2many:file_folders;constraint:OnDelete:CASCADE" json:"folders,omitempty"`
}
