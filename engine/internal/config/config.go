package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port             string
	DatabaseURL      string
	AppDatabaseURL   string
	RedisURL         string
	JWTSecret        []byte
	JWTIssuer        string
	JWTTTL           time.Duration
	SuperAdminEmail  string
	SuperAdminPasswd string
	LogLevel         string
	AllowedOrigins   []string

	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool
	RecordingsDir  string
	RetentionYears int
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
		MinioEndpoint:    os.Getenv("MINIO_ENDPOINT"),
		MinioAccessKey:   os.Getenv("MINIO_ACCESS_KEY"),
		MinioSecretKey:   os.Getenv("MINIO_SECRET_KEY"),
		MinioBucket:      getEnv("MINIO_BUCKET_RECORDINGS", "recordings"),
		MinioUseSSL:      os.Getenv("MINIO_USE_SSL") == "true",
		RecordingsDir:    getEnv("RECORDINGS_DIR", "/recordings"),
		RetentionYears:   atoiDefault(os.Getenv("RECORDING_RETENTION_YEARS"), 4),
	}
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return c, fmt.Errorf("JWT_SECRET must be at least 32 bytes (got %d)", len(secret))
	}
	c.JWTSecret = []byte(secret)

	ttlMin, _ := strconv.Atoi(getEnv("JWT_TTL_MINUTES", "60"))
	c.JWTTTL = time.Duration(ttlMin) * time.Minute

	if origins := os.Getenv("ALLOWED_ORIGINS"); origins != "" {
		for _, o := range strings.Split(origins, ",") {
			c.AllowedOrigins = append(c.AllowedOrigins, strings.TrimSpace(o))
		}
	}

	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}

	appURL := os.Getenv("APP_DATABASE_URL")
	if appURL == "" {
		appUser := getEnv("APP_DATABASE_USER", "app_user")
		appPass := getEnv("APP_DATABASE_PASSWORD", "app_user_change_me")
		derived, err := rewriteURLUser(c.DatabaseURL, appUser, appPass)
		if err != nil {
			return c, fmt.Errorf("derive app database url: %w", err)
		}
		appURL = derived
	}
	c.AppDatabaseURL = appURL

	return c, nil
}

func rewriteURLUser(connURL, user, password string) (string, error) {
	u, err := url.Parse(connURL)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(user, password)
	return u.String(), nil
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func atoiDefault(s string, d int) int {
	if s == "" {
		return d
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return n
}
