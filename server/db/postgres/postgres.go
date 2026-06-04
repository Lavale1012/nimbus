// Package postgres handles connecting to PostgreSQL via GORM and keeping the
// schema up to date with AutoMigrate.
package postgres

import (
	"fmt"
	"log"

	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connect reads DATABASE_URL from the environment, opens a GORM connection,
// and automatically creates/updates all tables to match the current model
// definitions. Returns the ready-to-use *gorm.DB handle.
func Connect() (*gorm.DB, error) {
	dsn, err := utils.GetEnv("DATABASE_URL")
	if err != nil {
		return nil, fmt.Errorf("failed to get DATABASE_URL from environment: %w", err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// AutoMigrate compares each model struct to the live schema and adds any
	// missing columns or tables. It never drops columns, so it's safe to run
	// on every startup.
	err = db.AutoMigrate(
		&models.User{},
		&models.Box{},
		&models.Folder{},
		&models.File{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-migrate database schema: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL and migrated schema")
	return db, nil
}
