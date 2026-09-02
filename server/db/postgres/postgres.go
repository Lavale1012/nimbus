// Package postgres handles connecting to PostgreSQL via GORM and keeping the
// schema up to date with AutoMigrate.
package postgres

import (
	"fmt"
	"log"
	"net/url"
	"strings"
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

// sslModes libpq treats as "encrypt, and fail if that isn't possible". Every
// other value, including the empty string — which libpq reads as "prefer" —
// falls back to an unencrypted connection without reporting an error.
var tlsRequiredSSLModes = map[string]bool{
	"require":     true,
	"verify-ca":   true,
	"verify-full": true,
}

// sslModeFrom extracts sslmode from a DSN in either form libpq accepts: the URL
// form (postgres://user:pass@host/db?sslmode=require) or the keyword/value form
// (host=... sslmode=require). Returns "" when the DSN sets no sslmode, and also
// when the DSN can't be parsed — an unreadable DSN is treated as "not proven to
// require TLS" rather than assumed safe.
func sslModeFrom(dsn string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return ""
		}
		return u.Query().Get("sslmode")
	}
	for _, field := range strings.Fields(dsn) {
		key, value, found := strings.Cut(field, "=")
		if found && strings.EqualFold(key, "sslmode") {
			return strings.Trim(value, `'"`)
		}
	}
	return ""
}

// checkTLS rejects a DSN that would let the connection fall back to plaintext.
//
// Defence in depth for the database layer: RDS enforces rds.force_ssl in its
// parameter group, but that lives in a different repo directory and a different
// apply. This makes the server refuse to start rather than silently sending the
// master credential over the network in the clear if the two ever disagree.
//
// LOCAL_DEV is exempt: docker-compose runs Postgres in a container with no
// certificate, so its DSN sets sslmode=disable.
func checkTLS(dsn string, localDev bool) error {
	if localDev {
		return nil
	}
	// Only the mode is ever echoed. The DSN itself holds the password.
	mode := strings.ToLower(strings.TrimSpace(sslModeFrom(dsn)))
	if tlsRequiredSSLModes[mode] {
		return nil
	}
	if mode == "" {
		return fmt.Errorf("DATABASE_URL sets no sslmode and LOCAL_DEV is not true: " +
			"libpq would treat this as \"prefer\" and silently accept an unencrypted " +
			"connection. Set sslmode to require, verify-ca, or verify-full")
	}
	return fmt.Errorf("DATABASE_URL sets sslmode=%q and LOCAL_DEV is not true: "+
		"this permits an unencrypted connection to the database. Set sslmode to "+
		"require, verify-ca, or verify-full", mode)
}

// Connect reads DATABASE_URL from the environment, opens a GORM connection,
// and automatically creates/updates all tables to match the current model
// definitions. Returns the ready-to-use *gorm.DB handle.
func Connect() (*gorm.DB, error) {
	dsn, err := utils.GetEnv("DATABASE_URL")
	if err != nil {
		return nil, fmt.Errorf("failed to get DATABASE_URL from environment: %w", err)
	}

	// Checked before the connection is opened, so a misconfigured deployment
	// fails at startup instead of after the first credential has crossed the
	// network unencrypted.
	localDev, _ := utils.GetEnv("LOCAL_DEV")
	if err := checkTLS(dsn, localDev == "true"); err != nil {
		return nil, err
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
