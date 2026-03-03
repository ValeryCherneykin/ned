// Package auth builds the SSH authentication method chain used when dialing.
//
// Priority order:
//  1. SSH agent (if $SSH_AUTH_SOCK is set)
//  2. Private key files (~/.ssh/id_ed25519, id_rsa, id_ecdsa)
//  3. Interactive password prompt (last resort)
package auth

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/ValeryCherneykin/ned/internal/terminal"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// defaultKeyFiles lists private key paths to try, in preference order.
var defaultKeyFiles = []string{
	"~/.ssh/id_ed25519",
	"~/.ssh/id_rsa",
	"~/.ssh/id_ecdsa",
}

// Methods assembles all available SSH auth methods for the given user.
// It always returns at least a password callback so the user is never
// left with an empty auth chain.
func Methods(user string) []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	// 1. SSH agent — zero friction when keys are already loaded.
	if m := agentMethod(); m != nil {
		methods = append(methods, m)
	}

	// 2. Private key files found on disk.
	for _, path := range defaultKeyFiles {
		if m := privateKeyMethod(path); m != nil {
			methods = append(methods, m)
		}
	}

	// 3. Password prompt — always appended as a guaranteed fallback.
	methods = append(methods, ssh.PasswordCallback(func() (string, error) {
		return terminal.PromptPassword(fmt.Sprintf("%s's password: ", user))
	}))

	return methods
}

// agentMethod returns an auth method backed by the local SSH agent socket.
// Returns nil if $SSH_AUTH_SOCK is not set or the socket cannot be reached.
func agentMethod() ssh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}

	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers)
}

// privateKeyMethod loads a single private key file and returns a PublicKeys
// auth method. Returns nil when the file is missing or cannot be parsed
// (e.g. passphrase-protected keys are skipped silently in act_1).
func privateKeyMethod(path string) ssh.AuthMethod {
	expanded := expandHome(path)

	data, err := os.ReadFile(expanded)
	if err != nil {
		return nil
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		// Passphrase-protected keys are handled in act_2.
		return nil
	}

	return ssh.PublicKeys(signer)
}

// expandHome replaces a leading "~" with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return filepath.Join(home, path[1:])
}
