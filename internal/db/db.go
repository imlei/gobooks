// 遵循project_guide.md
package db

import (
	"fmt"
	"log"
	"time"

	"balanciz/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	connectMaxAttempts = 5
	connectBaseDelay   = 2 * time.Second
)

// Connect creates a GORM connection to PostgreSQL, retrying up to 5 times with
// linear backoff (2 s, 4 s, 6 s, 8 s). Each attempt pings the database after
// opening so a TCP-level connection failure is caught immediately.
func Connect(cfg config.Config) (*gorm.DB, error) {
	connectTimeout := cfg.DBConnectTimeoutSeconds
	if connectTimeout <= 0 {
		connectTimeout = 5
	}
	maxOpenConns := cfg.DBMaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 25
	}
	maxIdleConns := cfg.DBMaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = 10
	}
	connMaxLifetimeSeconds := cfg.DBConnMaxLifetimeSeconds
	if connMaxLifetimeSeconds <= 0 {
		connMaxLifetimeSeconds = 1800
	}
	connMaxIdleTimeSeconds := cfg.DBConnMaxIdleTimeSeconds
	if connMaxIdleTimeSeconds <= 0 {
		connMaxIdleTimeSeconds = 300
	}
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
		connectTimeout,
	)

	// Keep logging simple and readable.
	gormLogger := logger.Default.LogMode(logger.Info)
	if cfg.Env == "prod" {
		gormLogger = logger.Default.LogMode(logger.Warn)
	}

	var lastErr error
	for attempt := 1; attempt <= connectMaxAttempts; attempt++ {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: gormLogger,
		})
		if err == nil {
			// Verify the connection is live with a ping.
			sqlDB, pingErr := db.DB()
			if pingErr == nil {
				pingErr = sqlDB.Ping()
			}
			if pingErr == nil {
				sqlDB.SetMaxOpenConns(maxOpenConns)
				sqlDB.SetMaxIdleConns(maxIdleConns)
				sqlDB.SetConnMaxLifetime(time.Duration(connMaxLifetimeSeconds) * time.Second)
				sqlDB.SetConnMaxIdleTime(time.Duration(connMaxIdleTimeSeconds) * time.Second)
				return db, nil
			}
			err = pingErr
		}

		lastErr = err
		if attempt < connectMaxAttempts {
			delay := time.Duration(attempt) * connectBaseDelay
			log.Printf("db connect attempt %d/%d failed (%v), retrying in %v",
				attempt, connectMaxAttempts, err, delay)
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf("db connect failed after %d attempts: %w", connectMaxAttempts, lastErr)
}
