package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalDriver(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "natscope-storage-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	driver := NewLocal(tmpDir)
	ctx := context.Background()

	// 1. Load non-existent
	_, err = driver.Load(ctx, "test.json")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// 2. Save
	data := []byte(`{"hello": "world"}`)
	if err := driver.Save(ctx, "test.json", data); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// 3. Load
	loaded, err := driver.Load(ctx, "test.json")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if string(loaded) != string(data) {
		t.Fatalf("expected %s, got %s", data, loaded)
	}

	// 4. List
	list, err := driver.List(ctx, "test")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 1 || filepath.Base(list[0]) != "test.json" {
		t.Fatalf("unexpected list output: %v", list)
	}

	// 5. Delete
	if err := driver.Delete(ctx, "test.json"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	_, err = driver.Load(ctx, "test.json")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestFactory(t *testing.T) {
	d, err := NewDriver(Config{Type: "local", StoragePath: "/tmp"})
	if err != nil || d.Name() != "local" {
		t.Fatalf("failed to create local driver: %v", err)
	}

	d2, err := NewDriver(Config{Type: "s3", S3Bucket: "my-bucket"})
	if err != nil || d2.Name() != "s3" {
		t.Fatalf("failed to create s3 driver: %v", err)
	}

	_, err = NewDriver(Config{Type: "invalid-type"})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

