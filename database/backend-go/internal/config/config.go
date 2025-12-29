package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DBConfig       DatabaseConfig
	ServerConfig   ServerConfig
	JWTSecret      string
	AllowedOrigins []string
	FileUpload     FileUploadConfig
	RateLimit      RateLimitConfig
	Email          EmailConfig
}

type DatabaseConfig struct {
	DSN                string
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetime    time.Duration
}

type ServerConfig struct {
	Port string
}

type FileUploadConfig struct {
	MaxSizeMB int64
}

type RateLimitConfig struct {
	RequestsPerMinute int
}

type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	FromAddress  string
	AppURL       string
}

// LoadConfig loads configuration from environment variables and validates them
func LoadConfig() (*Config, error) {
	cfg := &Config{
		DBConfig: DatabaseConfig{
			DSN:              getEnv("DB_DSN", "host=localhost user=user password=password dbname=converter_db port=5439 sslmode=disable"),
			MaxOpenConns:     getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:     getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime:  time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_MINUTES", 5)) * time.Minute,
		},
		ServerConfig: ServerConfig{
			Port: getEnv("PORT", "8080"),
		},
		JWTSecret: getEnv("JWT_SECRET", ""),
		FileUpload: FileUploadConfig{
			MaxSizeMB: int64(getEnvInt("MAX_UPLOAD_SIZE_MB", 10)),
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: getEnvInt("RATE_LIMIT_PER_MINUTE", 60),
		},
		Email: EmailConfig{
			SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
			SMTPPort:     getEnvInt("SMTP_PORT", 587),
			SMTPUser:     getEnv("SMTP_USER", ""),
			SMTPPassword: getEnv("SMTP_PASSWORD", ""),
			FromAddress:  getEnv("EMAIL_FROM", "noreply@yourapp.com"),
			AppURL:       getEnv("APP_URL", "http://localhost:8080"),
		},
	}

	// Parse ALLOWED_ORIGINS
	originsStr := getEnv("ALLOWED_ORIGINS", "")
	cfg.AllowedOrigins = parseCommaSeparated(originsStr)

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that all required configuration fields are set
func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters long for security")
	}
	if c.DBConfig.DSN == "" {
		return fmt.Errorf("DB_DSN is required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func parseCommaSeparated(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if i > start {
				part := strings.TrimSpace(s[start:i])
				if part != "" {
					result = append(result, part)
				}
			}
			start = i + 1
		}
	}
	if start < len(s) {
		part := strings.TrimSpace(s[start:])
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
