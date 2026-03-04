package mock_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/ValeryCherneykin/ned/internal/backend/mock"
)

func TestBackend_ReadWrite(t *testing.T) {
	t.Parallel()

	content := []byte("hello from mock")
	b := mock.New(map[string][]byte{"/file.txt": content})

	rc, err := b.ReadFile("/file.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	defer func() {
		if closeErr := rc.Close(); closeErr != nil {
			t.Errorf("Close: %v", closeErr)
		}
	}()

	got, readErr := io.ReadAll(rc)
	if readErr != nil {
		t.Fatalf("ReadAll: %v", readErr)
	}

	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestBackend_NotExist(t *testing.T) {
	t.Parallel()

	b := mock.New(nil)

	_, err := b.ReadFile("/no-such-file")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected ErrNotExist, got: %v", err)
	}
}

func TestBackend_WriteAndRead(t *testing.T) {
	t.Parallel()

	b := mock.New(nil)
	content := []byte("written content")

	if err := b.WriteFile("/new.txt", bytes.NewReader(content)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if !b.Has("/new.txt") {
		t.Error("Has(/new.txt) = false after write")
	}

	if !bytes.Equal(b.Get("/new.txt"), content) {
		t.Error("Get content mismatch after write")
	}
}

func TestBackend_Close(t *testing.T) {
	t.Parallel()

	b := mock.New(nil)

	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if !b.Closed {
		t.Error("Closed = false after Close()")
	}
}

func TestBackend_MkdirAllTracking(t *testing.T) {
	t.Parallel()

	b := mock.New(nil)

	if err := b.MkdirAll("/some/deep/path"); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if !b.HasMkdirAll("/some/deep/path") {
		t.Error("HasMkdirAll(/some/deep/path) = false after call")
	}
}

func TestBackend_WriteErr(t *testing.T) {
	t.Parallel()

	b := mock.New(nil)
	b.WriteErr = os.ErrPermission

	if err := b.WriteFile("/f", bytes.NewReader([]byte("x"))); err == nil {
		t.Error("WriteFile expected error, got nil")
	}
}

func TestBackend_ReadErr(t *testing.T) {
	t.Parallel()

	b := mock.New(map[string][]byte{"/f": []byte("x")})
	b.ReadErr = os.ErrPermission

	if _, err := b.ReadFile("/f"); err == nil {
		t.Error("ReadFile expected error, got nil")
	}
}
