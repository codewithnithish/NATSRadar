package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocalDriver stores files on the local filesystem (including mounted PV, PVC, and NFS volumes).
type LocalDriver struct {
	baseDir string
}

// NewLocal creates a local filesystem storage driver.
func NewLocal(baseDir string) *LocalDriver {
	if baseDir == "" {
		baseDir = "."
	}
	return &LocalDriver{baseDir: baseDir}
}

func (l *LocalDriver) Name() string {
	return "local"
}

func (l *LocalDriver) resolve(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(l.baseDir, p)
}

func (l *LocalDriver) Load(_ context.Context, p string) ([]byte, error) {
	fullPath := l.resolve(p)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("local load %s: %w", fullPath, err)
	}
	return data, nil
}

func (l *LocalDriver) Save(_ context.Context, p string, data []byte) error {
	fullPath := l.resolve(p)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}

	// Atomic write via temp file
	tmp := fmt.Sprintf("%s.tmp.%d", fullPath, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write tmp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, fullPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("atomic rename %s -> %s: %w", tmp, fullPath, err)
	}
	return nil
}

func (l *LocalDriver) Delete(_ context.Context, p string) error {
	fullPath := l.resolve(p)
	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("local delete %s: %w", fullPath, err)
	}
	return nil
}

func (l *LocalDriver) List(_ context.Context, prefix string) ([]string, error) {
	fullPrefix := l.resolve(prefix)
	dir := filepath.Dir(fullPrefix)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var matched []string
	baseName := filepath.Base(fullPrefix)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), baseName) {
			matched = append(matched, filepath.Join(dir, e.Name()))
		}
	}
	return matched, nil
}

func (l *LocalDriver) Close() error {
	return nil
}
