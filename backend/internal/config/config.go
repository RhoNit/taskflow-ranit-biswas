package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port      string
	DB        DBConfig
	JWTSecret string
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

func Load() *Config {
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))

	return &Config{
		Port: getEnv("PORT", "8066"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "taskflow"),
			Password: getEnv("DB_PASSWORD", "taskflow"),
			Name:     getEnv("DB_NAME", "taskflow"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWTSecret: getEnv("JWT_SECRET", "change-me-in-production"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
