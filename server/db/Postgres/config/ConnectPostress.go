package postgres

import (
	"fmt"
	"time"

	"github.com/nimbus/api/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func ConnectPostgres() (*gorm.DB, error) {

	// Try to use DB_DSN first (for local dev), fallback to individual vars (for AWS RDS)
	connStr, err := utils.GetEnv("DB_DSN")
	if err != nil || connStr == "" {
		return nil, fmt.Errorf("DB_DSN environment variable is not set")
	}
	// Configure GORM
	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to open database connection: %v", err)
	}

	// Get underlying SQL database to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %v", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	fmt.Println("Successfully connected to PostgreSQL with GORM!")
	return db, nil
}
