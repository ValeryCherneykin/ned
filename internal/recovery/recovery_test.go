package recovery_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ValeryCherneykin/ned/internal/recovery"
)

func TestSave_CreatesRecoveryFile(t *testing.T) {
	// Point HOME to temp dir so we don't pollute ~/.ned/recovery.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create a fake temp file with content.
	tmp, err := os.CreateTemp("", "ned-recovery-test-")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	defer os.Remove(tmp.Name())

	content := []byte("SECRET=abc123\n")

	if _, err = tmp.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	tmp.Close()

	saved, err := recovery.Save(tmp.Name(), "/etc/.env")
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// File must exist.
	if _, err = os.Stat(saved); err != nil {
		t.Fatalf("recovery file not found: %v", err)
	}

	// File must contain original content.
	got, err := os.ReadFile(saved)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestSave_FilenameContainsBasename(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	tmp, err := os.CreateTemp("", "ned-")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	defer os.Remove(tmp.Name())
	tmp.Close()

	saved, err := recovery.Save(tmp.Name(), "/app/config.yml")
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	base := filepath.Base(saved)
	if !strings.HasPrefix(base, "config.yml") {
		t.Errorf("recovery filename %q does not start with remote basename", base)
	}
}

func TestSave_FilenameContainsTimestamp(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	tmp, err := os.CreateTemp("", "ned-")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	defer os.Remove(tmp.Name())
	tmp.Close()

	saved, err := recovery.Save(tmp.Name(), "/tmp/test.txt")
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Timestamp format: 20060102_150405 — 15 chars
	base := filepath.Base(saved)
	if len(base) < len("test.txt_20060102_150405") {
		t.Errorf("recovery filename %q missing timestamp", base)
	}
}

func TestSave_FilePermissions(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	tmp, err := os.CreateTemp("", "ned-")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	defer os.Remove(tmp.Name())
	tmp.Close()

	saved, err := recovery.Save(tmp.Name(), "/etc/secret")
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	info, err := os.Stat(saved)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	// Recovery files must be readable only by owner.
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want 0600", perm)
	}
}
