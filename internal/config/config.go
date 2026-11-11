// Package config loads Natscope runtime configuration from environment variables.
package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds process-level settings.
type Config struct {
	// Addr is the HTTP listen address, e.g. ":8080".
	Addr string
	// NATSURL is the URL of the default NATS connection created at startup.
	// Leave empty (NATS_URL="") to start with no connection and add one from the UI.
	NATSURL string
	// DefaultConnectionName is the display name of the default connection.
	DefaultConnectionName string
	// ReadOnly marks the default connection read-only.
	ReadOnly bool
	// LogLevel is one of debug, info, warn, error.
	LogLevel slog.Level
	// StoragePath is the file path where added connections are persisted.
	StoragePath string
	// StorageType specifies the storage provider: local, s3, nfs, pvc.
	StorageType string
	// S3 Configuration (Amazon S3, MinIO, Cloudflare R2, Ceph)
	S3Endpoint     string
	S3Bucket       string
	S3Prefix       string
	S3Region       string
	S3AccessKey    string
	S3SecretKey    string
	S3UseSSL       bool
	S3StorageClass string
	S3TTLDays      int
	S3MaxBytes     int64
	// AuthEnabled turns on UI/API authentication.
	AuthEnabled bool
	// AuthUser is the administrative login username.
	AuthUser string
	// AuthPass is the administrative login password.
	AuthPass string
}

// FromEnv reads configuration from the environment, applying defaults.
func FromEnv() Config {
	addr := getenv("NATSCOPE_ADDR", "")
	if addr == "" {
		if p, ok := os.LookupEnv("PORT"); ok && p != "" {
			addr = ":" + p
		} else {
			addr = ":8080"
		}
	}

	return Config{
		Addr:                  addr,
		NATSURL:               getenv("NATS_URL", "nats://127.0.0.1:4222"),
		DefaultConnectionName: getenv("NATSCOPE_DEFAULT_CONNECTION_NAME", "default"),
		ReadOnly:              getbool("NATSCOPE_READ_ONLY", false),
		LogLevel:              parseLevel(getenv("NATSCOPE_LOG_LEVEL", "info")),
		StoragePath:           getenv("NATSCOPE_STORAGE_PATH", ".data/connections.json"),
		StorageType:           getenv("NATSCOPE_STORAGE_TYPE", "local"),
		S3Endpoint:            getenv("NATSCOPE_S3_ENDPOINT", ""),
		S3Bucket:              getenv("NATSCOPE_S3_BUCKET", ""),
		S3Prefix:              getenv("NATSCOPE_S3_PREFIX", ""),
		S3Region:              getenv("NATSCOPE_S3_REGION", "us-east-1"),
		S3AccessKey:           getenv("NATSCOPE_S3_ACCESS_KEY", ""),
		S3SecretKey:           getenv("NATSCOPE_S3_SECRET_KEY", ""),
		S3UseSSL:              getbool("NATSCOPE_S3_USE_SSL", true),
		S3StorageClass:        getenv("NATSCOPE_S3_STORAGE_CLASS", "STANDARD"),
		S3TTLDays:             getint("NATSCOPE_S3_TTL_DAYS", 0),
		S3MaxBytes:            getint64("NATSCOPE_S3_MAX_BYTES", 0),
		AuthEnabled:           getbool("NATSCOPE_AUTH_ENABLED", true),
		AuthUser:              getenv("NATSCOPE_AUTH_USER", "admin"),
		AuthPass:              getenv("NATSCOPE_AUTH_PASS", "admin@123"),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func getbool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return b
}

func getint(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

func getint64(key string, def int64) int64 {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return def
	}
	return n
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
