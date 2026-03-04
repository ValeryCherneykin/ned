package keygen_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ValeryCherneykin/ned/internal/keygen"
)

func TestEnsureKeyPair_GeneratesNewKeys(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	kp, err := keygen.EnsureKeyPair(io.Discard)
	if err != nil {
		t.Fatalf("EnsureKeyPair() error: %v", err)
	}

	for _, path := range []string{kp.PrivatePath, kp.PublicPath} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("expected file %q to exist: %v", path, statErr)
		}
	}
}

func TestEnsureKeyPair_Idempotent(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	kp1, err := keygen.EnsureKeyPair(io.Discard)
	if err != nil {
		t.Fatalf("first EnsureKeyPair() error: %v", err)
	}

	first, err := os.ReadFile(kp1.PrivatePath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}

	kp2, err := keygen.EnsureKeyPair(io.Discard)
	if err != nil {
		t.Fatalf("second EnsureKeyPair() error: %v", err)
	}

	second, err := os.ReadFile(kp2.PrivatePath)
	if err != nil {
		t.Fatalf("read key second: %v", err)
	}

	// Key must not be regenerated on second call.
	if !bytes.Equal(first, second) {
		t.Error("EnsureKeyPair() regenerated key on second call — should be idempotent")
	}
}

func TestEnsureKeyPair_PrivateKeyPermissions(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	kp, err := keygen.EnsureKeyPair(io.Discard)
	if err != nil {
		t.Fatalf("EnsureKeyPair() error: %v", err)
	}

	info, err := os.Stat(kp.PrivatePath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("private key permissions = %o, want 0600", perm)
	}
}

func TestEnsureKeyPair_PublicKeyFormat(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	kp, err := keygen.EnsureKeyPair(io.Discard)
	if err != nil {
		t.Fatalf("EnsureKeyPair() error: %v", err)
	}

	pub, err := os.ReadFile(kp.PublicPath)
	if err != nil {
		t.Fatalf("read public key: %v", err)
	}

	if !strings.HasPrefix(string(pub), "ssh-ed25519 ") {
		// Show at most 40 chars of the unexpected prefix.
		preview := string(pub)
		if len(preview) > 40 {
			preview = preview[:40]
		}

		t.Errorf("public key format unexpected: %q", preview)
	}
}

func TestDefaultKeyPair_Paths(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	kp, err := keygen.DefaultKeyPair()
	if err != nil {
		t.Fatalf("DefaultKeyPair() error: %v", err)
	}

	wantPriv := filepath.Join(tmpHome, ".ssh", keygen.KeyName)
	wantPub := wantPriv + ".pub"

	if kp.PrivatePath != wantPriv {
		t.Errorf("PrivatePath = %q, want %q", kp.PrivatePath, wantPriv)
	}

	if kp.PublicPath != wantPub {
		t.Errorf("PublicPath = %q, want %q", kp.PublicPath, wantPub)
	}
}

func TestInstallOnServer_ScriptContent(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	kp, err := keygen.EnsureKeyPair(io.Discard)
	if err != nil {
		t.Fatalf("EnsureKeyPair() error: %v", err)
	}

	var capturedCmd string

	runner := func(cmd string) (string, error) {
		capturedCmd = cmd
		return "", nil
	}

	if err = keygen.InstallOnServer(kp.PublicPath, runner); err != nil {
		t.Fatalf("InstallOnServer() error: %v", err)
	}

	if !strings.Contains(capturedCmd, "authorized_keys") {
		t.Errorf("install script does not mention authorized_keys:\n%s", capturedCmd)
	}

	if !strings.Contains(capturedCmd, "grep") {
		t.Errorf("install script missing idempotency check (grep):\n%s", capturedCmd)
	}
}
