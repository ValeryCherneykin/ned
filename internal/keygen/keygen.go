// Package keygen handles ed25519 key pair generation and server-side installation.
// It is called after a successful password-auth connection to offer
// passwordless access on future connects.
package keygen

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	// KeyName is the private key file ned generates.
	KeyName = "ned_id_ed25519"
)

// KeyPair holds the local paths to the generated key files.
type KeyPair struct {
	PrivatePath string
	PublicPath  string
}

// DefaultKeyPair returns the expected paths for ned's managed key pair.
func DefaultKeyPair() (KeyPair, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return KeyPair{}, fmt.Errorf("resolve home: %w", err)
	}

	base := filepath.Join(home, ".ssh", KeyName)

	return KeyPair{PrivatePath: base, PublicPath: base + ".pub"}, nil
}

// EnsureKeyPair generates ~/.ssh/ned_id_ed25519 if it does not exist.
// Status messages are written to out — pass os.Stdout for normal use,
// io.Discard in tests to suppress output.
func EnsureKeyPair(out io.Writer) (KeyPair, error) {
	kp, err := DefaultKeyPair()
	if err != nil {
		return KeyPair{}, err
	}

	// Already exists — nothing to do.
	if _, err = os.Stat(kp.PrivatePath); err == nil {
		return kp, nil
	}

	if err = generate(kp); err != nil {
		return KeyPair{}, fmt.Errorf("generate key pair: %w", err)
	}

	fmt.Fprintf(out, "✓ generated new SSH key: %s\n", kp.PrivatePath)

	return kp, nil
}

// InstallOnServer appends the public key to ~/.ssh/authorized_keys on the
// remote host using an already-open SSH session runner.
// runner executes a remote command and returns its output.
func InstallOnServer(pubKeyPath string, runner func(cmd string) (string, error)) error {
	pubData, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("read public key %s: %w", pubKeyPath, err)
	}

	pubLine := strings.TrimSpace(string(pubData))

	// Idempotent install: only append if not already present.
	script := fmt.Sprintf(
		`mkdir -p ~/.ssh && chmod 700 ~/.ssh && `+
			`grep -qF %q ~/.ssh/authorized_keys 2>/dev/null || `+
			`echo %q >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`,
		pubLine, pubLine,
	)

	if _, err = runner(script); err != nil {
		return fmt.Errorf("install public key on server: %w", err)
	}

	return nil
}

// generate creates an ed25519 key pair and writes the files to disk.
func generate(kp KeyPair) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ed25519 key: %w", err)
	}

	privPEM, err := ssh.MarshalPrivateKey(priv, "ned managed key")
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}

	// Create ~/.ssh/ if it does not exist yet.
	if err = os.MkdirAll(filepath.Dir(kp.PrivatePath), 0o700); err != nil {
		return fmt.Errorf("create .ssh dir: %w", err)
	}

	if err = os.WriteFile(kp.PrivatePath, pem.EncodeToMemory(privPEM), 0o600); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return fmt.Errorf("create ssh public key: %w", err)
	}

	if err = os.WriteFile(kp.PublicPath, ssh.MarshalAuthorizedKey(sshPub), 0o644); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}

	return nil
}
