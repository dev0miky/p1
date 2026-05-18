package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port             string
	DatabaseURL      string
	RedisURL         string
	JWTSecret        []byte
	JWTIssuer        string
	JWTTTL           time.Duration
	SuperAdminEmail  string
	SuperAdminPasswd string
	LogLevel         string
}

func Load() (Config, error) {
	c := Config{
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		RedisURL:         os.Getenv("REDIS_URL"),
		JWTIssuer:        getEnv("JWT_ISSUER", "p1"),
		SuperAdminEmail:  os.Getenv("SUPER_ADMIN_EMAIL"),
		SuperAdminPasswd: os.Getenv("SUPER_ADMIN_PASSWORD"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
	}
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return c, fmt.Errorf("JWT_SECRET must be at least 32 bytes (got %d)", len(secret))
	}
	c.JWTSecret = []byte(secret)

	ttlMin, _ := strconv.Atoi(getEnv("JWT_TTL_MINUTES", "60"))
	c.JWTTTL = time.Duration(ttlMin) * time.Minute

	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
