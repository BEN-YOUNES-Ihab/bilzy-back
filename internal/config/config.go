package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL       string
	FirebaseProjectID string
	GoogleCredsPath   string
	Port              string
	CORSOrigins       []string
	LogLevel          slog.Level
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	c := &Config{
		DatabaseURL:       strings.TrimSpace(os.Getenv("DATABASE_URL")),
		FirebaseProjectID: strings.TrimSpace(os.Getenv("FIREBASE_PROJECT_ID")),
		GoogleCredsPath:   strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")),
		Port:              strings.TrimSpace(os.Getenv("PORT")),
		CORSOrigins:       splitCSV(os.Getenv("CORS_ORIGINS")),
		LogLevel:          parseLogLevel(os.Getenv("LOG_LEVEL")),
	}
	if c.Port == "" {
		c.Port = "8080"
	}

	if c.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if c.FirebaseProjectID == "" {
		return nil, errors.New("FIREBASE_PROJECT_ID is required")
	}

	return c, nil
}

func (c *Config) Addr() string {
	if strings.HasPrefix(c.Port, ":") {
		return c.Port
	}
	return fmt.Sprintf(":%s", c.Port)
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
