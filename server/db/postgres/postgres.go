// Package postgres handles connecting to PostgreSQL via GORM and keeping the
// schema up to date with AutoMigrate.
package postgres

import (
	"fmt"
	"log"
	"time"

	"github.com/nimbus/api/models"
	"github.com/nimbus/api/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connection-pool defaults. These are conservative for a small RDS instance
// (e.g. db.t3.micro, ~85 max connections) shared by a couple of API tasks:
// with maxOpenConns=20 per task, two tasks stay well under the server limit
// while leaving headroom for migrations and admin connections.
const (
	maxOpenConns    = 20
	maxIdleConns    = 10
	connMaxLifetime = 30 * time.Minute
	connMaxIdleTime = 5 * time.Minute
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

	// Configure the underlying connection pool. Without this GORM uses Go's
	// database/sql defaults (unlimited open connections), which can exhaust a
	// small RDS instance under load. Bounding the pool also fails fast instead
	// of piling up connections when the DB is the bottleneck.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

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
