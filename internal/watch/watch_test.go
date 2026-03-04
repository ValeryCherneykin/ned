package watch_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/ValeryCherneykin/ned/internal/backend/mock"
	"github.com/ValeryCherneykin/ned/internal/watch"
)

func TestWatch_UploadsOnWrite(t *testing.T) {
	t.Parallel()

	// Create a temp file to watch.
	tmp, err := os.CreateTemp("", "ned-watch-test-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	t.Cleanup(func() { _ = os.Remove(tmp.Name()) })

	if err = tmp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b := mock.New(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- watch.Watch(ctx, tmp.Name(), "/remote/file.txt", b)
	}()

	// Give watcher time to start.
	time.Sleep(100 * time.Millisecond)

	// Write to the file — simulates :w in vim.
	content := []byte("hello from watch test")
	if err = os.WriteFile(tmp.Name(), content, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wait longer than the 500ms poll interval so the watcher detects the change.
	time.Sleep(1200 * time.Millisecond)

	cancel()

	select {
	case err = <-errCh:
		if err != nil {
			t.Errorf("Watch() error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Watch() did not stop after ctx cancel")
	}

	got := b.Get("/remote/file.txt")
	if !bytes.Equal(got, content) {
		t.Errorf("remote content = %q, want %q", got, content)
	}
}

func TestWatch_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	tmp, err := os.CreateTemp("", "ned-watch-cancel-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	t.Cleanup(func() { _ = os.Remove(tmp.Name()) })

	if err = tmp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b := mock.New(nil)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)

	go func() {
		errCh <- watch.Watch(ctx, tmp.Name(), "/remote/file.txt", b)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err = <-errCh:
		if err != nil {
			t.Errorf("Watch() error after cancel: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Watch() did not stop after ctx cancel")
	}
}

func TestWatch_WriteError(t *testing.T) {
	t.Parallel()

	tmp, err := os.CreateTemp("", "ned-watch-werr-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	t.Cleanup(func() { _ = os.Remove(tmp.Name()) })

	if err = tmp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b := mock.New(nil)
	b.WriteErr = os.ErrPermission // upload will fail

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- watch.Watch(ctx, tmp.Name(), "/remote/file.txt", b)
	}()

	time.Sleep(100 * time.Millisecond)

	// Trigger a write — upload should fail but watcher should keep running.
	if err = os.WriteFile(tmp.Name(), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Watcher should still be alive — cancel it cleanly.
	cancel()

	select {
	case err = <-errCh:
		if err != nil {
			t.Errorf("Watch() should survive upload error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Watch() did not stop after ctx cancel")
	}
}
