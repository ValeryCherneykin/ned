package dirmode_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ValeryCherneykin/ned/internal/backend/mock"
	"github.com/ValeryCherneykin/ned/internal/dirmode"
)

func TestDownload_CreatesLocalFiles(t *testing.T) {
	t.Parallel()

	b := mock.New(map[string][]byte{
		"/etc/nginx/nginx.conf":            []byte("worker_processes 1;"),
		"/etc/nginx/sites-enabled/default": []byte("server {}"),
	})

	localDir := t.TempDir()

	if err := dirmode.Download(b, "/etc/nginx", localDir); err != nil {
		t.Fatalf("Download() error: %v", err)
	}

	for _, rel := range []string{"nginx.conf", "sites-enabled/default"} {
		path := filepath.Join(localDir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist: %v", rel, err)
		}
	}
}

func TestSnapshot_CapturesMtimes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	snap, err := dirmode.Snapshot(dir)
	if err != nil {
		t.Fatalf("Snapshot() error: %v", err)
	}

	if _, ok := snap["a.txt"]; !ok {
		t.Error("snapshot missing a.txt")
	}
}

func TestCollectDeleted(t *testing.T) {
	t.Parallel()

	now := time.Now()

	before := map[string]dirmode.FileInfo{
		"nginx.conf": {ModTime: now},
		"removed":    {ModTime: now},
	}

	after := map[string]dirmode.FileInfo{
		"nginx.conf": {ModTime: now},
	}

	deleted := dirmode.CollectDeleted(before, after)

	if len(deleted) != 1 || deleted[0] != "removed" {
		t.Errorf("CollectDeleted = %v, want [removed]", deleted)
	}
}

func TestWatch_UploadsChangedFile(t *testing.T) {
	t.Parallel()

	b := mock.New(nil)
	dir := t.TempDir()

	// Create initial file.
	filePath := filepath.Join(dir, "test.conf")

	if err := os.WriteFile(filePath, []byte("v1"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- dirmode.Watch(ctx, dir, "/remote", b)
	}()

	// Wait for first poll cycle.
	time.Sleep(200 * time.Millisecond)

	// Modify the file.
	if err := os.WriteFile(filePath, []byte("v2"), 0o600); err != nil {
		t.Fatalf("WriteFile v2: %v", err)
	}

	// Wait for poll to detect change.
	time.Sleep(1200 * time.Millisecond)

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Watch() error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Watch() did not stop after ctx cancel")
	}

	got := b.Get("/remote/test.conf")
	if string(got) != "v2" {
		t.Errorf("remote content = %q, want %q", got, "v2")
	}
}
