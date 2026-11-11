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
