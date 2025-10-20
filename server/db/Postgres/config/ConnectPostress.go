package config

import (
	"fmt"
	"log"

	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnectPostgres() (*gorm.DB, error) {
	dsn, err := utils.GetEnv("DATABASE_URL")
	if err != nil {
		return nil, fmt.Errorf("failed to get DATABASE_URL from environment: %w", err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(
		&models.UserModel{},
		&models.BoxModel{},
		&models.FolderModel{},
		&models.FileModel{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-migrate database schema: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL and migrated schema")
	return db, nil
}
