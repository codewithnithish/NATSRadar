package storage

import (
	"fmt"
	"strings"
)

// NewDriver constructs the requested storage backend driver.
func NewDriver(cfg Config) (Driver, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "", "local", "file", "pvc", "nfs", "pv", "hostpath":
		return NewLocal(cfg.StoragePath), nil

	case "s3", "minio", "r2", "wasabi", "ceph":
		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("s3 storage requires a bucket name")
		}
		return NewS3(
			cfg.S3Endpoint,
			cfg.S3Bucket,
			cfg.S3Region,
			cfg.S3AccessKey,
			cfg.S3SecretKey,
			cfg.S3UseSSL,
		), nil

	default:
		return nil, fmt.Errorf("unsupported storage type: %s (valid: local, s3, nfs, pvc)", cfg.Type)
	}
}
