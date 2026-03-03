package keygen_test

import (
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

	// Both files must exist.
	for _, path := range []string{kp.PrivatePath, kp.PublicPath} {
		if _, err = os.Stat(path); err != nil {
			t.Errorf("expected file %q to exist: %v", path, err)
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

	// Read private key content before second call.
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
	if string(first) != string(second) {
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

	// Private key must be 0600 — readable only by owner.
	perm := info.Mode().Perm()
	if perm != 0o600 {
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

	// authorized_keys format starts with the key type.
	if !strings.HasPrefix(string(pub), "ssh-ed25519 ") {
		t.Errorf("public key format unexpected: %q", string(pub)[:min(len(pub), 40)])
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

	// Script must reference authorized_keys.
	if !strings.Contains(capturedCmd, "authorized_keys") {
		t.Errorf("install script does not mention authorized_keys:\n%s", capturedCmd)
	}

	// Script must be idempotent (grep check).
	if !strings.Contains(capturedCmd, "grep") {
		t.Errorf("install script missing idempotency check (grep):\n%s", capturedCmd)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
