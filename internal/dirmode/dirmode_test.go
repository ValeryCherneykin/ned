package dirmode_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ValeryCherneykin/ned/internal/backend/mock"
	"github.com/ValeryCherneykin/ned/internal/dirmode"
	"github.com/ValeryCherneykin/ned/internal/ignore"
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

func TestDownload_RespectsNedIgnore(t *testing.T) {
	t.Parallel()

	b := mock.New(map[string][]byte{
		"/app/.nedignore":            []byte("node_modules/\n*.log\n"),
		"/app/main.go":               []byte("package main"),
		"/app/node_modules/index.js": []byte("module.exports = {}"),
		"/app/error.log":             []byte("some error"),
	})

	localDir := t.TempDir()

	if err := dirmode.Download(b, "/app", localDir); err != nil {
		t.Fatalf("Download() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(localDir, "main.go")); err != nil {
		t.Error("main.go should exist")
	}

	if _, err := os.Stat(filepath.Join(localDir, "node_modules")); err == nil {
		t.Error("node_modules should be ignored")
	}

	if _, err := os.Stat(filepath.Join(localDir, "error.log")); err == nil {
		t.Error("error.log should be ignored")
	}
}

func TestSnapshot_CapturesMtimes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := ignore.ParseString("")

	snap, err := dirmode.Snapshot(dir, m)
	if err != nil {
		t.Fatalf("Snapshot() error: %v", err)
	}

	if _, ok := snap["a.txt"]; !ok {
		t.Error("snapshot missing a.txt")
	}
}

func TestSnapshot_RespectsIgnore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "app.log"), []byte("log"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := ignore.ParseString("*.log\n")

	snap, err := dirmode.Snapshot(dir, m)
	if err != nil {
		t.Fatalf("Snapshot() error: %v", err)
	}

	if _, ok := snap["main.go"]; !ok {
		t.Error("main.go should be in snapshot")
	}

	if _, ok := snap["app.log"]; ok {
		t.Error("app.log should be excluded by ignore")
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

	filePath := filepath.Join(dir, "test.conf")

	if err := os.WriteFile(filePath, []byte("v1"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := ignore.ParseString("")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- dirmode.Watch(ctx, dir, "/remote", b, m)
	}()

	time.Sleep(200 * time.Millisecond)

	if err := os.WriteFile(filePath, []byte("v2"), 0o600); err != nil {
		t.Fatalf("WriteFile v2: %v", err)
	}

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

func TestUploadChanged_OnlyChanged(t *testing.T) {
	t.Parallel()

	b := mock.New(nil)
	dir := t.TempDir()

	now := time.Now()
	later := now.Add(time.Second)

	if err := os.WriteFile(filepath.Join(dir, "changed.go"), []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "unchanged.go"), []byte("old"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	before := map[string]dirmode.FileInfo{
		"changed.go":   {ModTime: now},
		"unchanged.go": {ModTime: now},
	}

	after := map[string]dirmode.FileInfo{
		"changed.go":   {ModTime: later},
		"unchanged.go": {ModTime: now}, // same mtime — not changed
	}

	if err := dirmode.UploadChanged(b, dir, "/remote", before, after); err != nil {
		t.Fatalf("UploadChanged() error: %v", err)
	}

	if !b.Has("/remote/changed.go") {
		t.Error("changed.go should have been uploaded")
	}

	if b.Has("/remote/unchanged.go") {
		t.Error("unchanged.go should NOT have been uploaded")
	}
}
