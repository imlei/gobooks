// 遵循产品需求 v1.0
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all application configuration in one place.
// Keep it small and obvious for beginners.
type Config struct {
	Env  string
	Addr string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
}

// Load reads .env (if present) and then reads environment variables.
// Environment variables always win.
func Load() (Config, error) {
	// .env is optional (nice for local dev). If it doesn't exist, ignore.
	_ = godotenv.Load()

	cfg := Config{
		Env:        getenv("APP_ENV", "dev"),
		Addr:       getenv("APP_ADDR", ":6768"),
		DBHost:     getenv("DB_HOST", "localhost"),
		DBPort:     getenv("DB_PORT", "5432"),
		DBUser:     getenv("DB_USER", "gobooks"),
		DBPassword: getenv("DB_PASSWORD", "gobooks"),
		DBName:     getenv("DB_NAME", "gobooks"),
		DBSSLMode:  getenv("DB_SSLMODE", "disable"),
	}

	if cfg.DBHost == "" || cfg.DBPort == "" || cfg.DBUser == "" || cfg.DBName == "" {
		return Config{}, fmt.Errorf("missing required DB config")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

