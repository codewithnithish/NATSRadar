// Package storage provides pluggable storage drivers for NATSRadar.
// Supported backends:
// - Local filesystem (Local disk, Kubernetes PV, PVC, and NFS mounts)
// - Amazon S3 / MinIO / S3-compatible Object Storage
// - SFTP (SSH File Transfer Protocol)
// - FTP (File Transfer Protocol)
package storage

import (
	"context"
	"errors"
)

var (
	// ErrNotFound is returned when a requested file or object does not exist.
	ErrNotFound = errors.New("file not found")
)

// Driver defines the common interface for all storage providers.
type Driver interface {
	// Name returns the driver identifier (e.g. "local", "s3", "sftp", "ftp").
	Name() string
	// Load retrieves raw bytes from the target path.
	Load(ctx context.Context, path string) ([]byte, error)
	// Save writes raw bytes to the target path.
	Save(ctx context.Context, path string, data []byte) error
	// Delete removes the file at the target path.
	Delete(ctx context.Context, path string) error
	// List returns matching file paths under the given prefix.
	List(ctx context.Context, prefix string) ([]string, error)
	// Close releases any network pools or open connections.
	Close() error
}

// Config holds settings for all supported storage drivers.
type Config struct {
	Type        string // "local", "s3", "sftp", "ftp"
	StoragePath string // Base path / directory (default ".data/connections.json")

	// S3 Configuration (AWS S3, MinIO, Wasabi, Ceph, Cloudflare R2)
	S3Endpoint     string // S3 host/endpoint (e.g. "s3.amazonaws.com", "minio.default.svc:9000")
	S3Bucket       string // Configurable bucket name
	S3Prefix       string // Subfolder or key prefix (e.g. "natscope/")
	S3Region       string // S3 region (default "us-east-1")
	S3AccessKey    string // AWS / MinIO Access Key ID
	S3SecretKey    string // AWS / MinIO Secret Key
	S3UseSSL       bool   // HTTPS (true) or HTTP (false)
	S3StorageClass string // e.g. "STANDARD", "STANDARD_IA", "INTELLIGENT_TIERING", "GLACIER_IR"
	S3TTLDays      int    // Lifecycle / object expiration in days (0 = no expiration)
	S3MaxBytes     int64  // Maximum bucket storage quota in bytes (0 = unlimited)

	// SFTP Configuration
	SFTPHost     string
	SFTPUser     string
	SFTPPassword string
	SFTPKey      string

	// FTP Configuration
	FTPHost     string
	FTPUser     string
	FTPPassword string
}
