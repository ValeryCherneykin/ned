// Package auth builds the SSH authentication method chain used when dialing.
//
// Priority order:
//  1. SSH agent ($SSH_AUTH_SOCK)
//  2. Private key files (~/.ssh/ned_id_ed25519, id_ed25519, id_rsa, id_ecdsa)
//  3. Interactive password prompt (characters hidden via x/term)
package auth

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/ValeryCherneykin/ned/internal/terminal"
)

var defaultKeyFiles = []string{
	"~/.ssh/ned_id_ed25519", // ned managed key — checked first
	"~/.ssh/id_ed25519",
	"~/.ssh/id_rsa",
	"~/.ssh/id_ecdsa",
}

// Methods assembles SSH auth methods for the given user.
// If identityFile is non-empty it is tried first, before the defaults.
func Methods(user, identityFile string) []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	if m := agentMethod(); m != nil {
		methods = append(methods, m)
	}

	// Explicit -i flag takes priority over defaults.
	if identityFile != "" {
		if m := privateKeyMethod(identityFile); m != nil {
			methods = append(methods, m)
		}
	} else {
		for _, path := range defaultKeyFiles {
			if m := privateKeyMethod(path); m != nil {
				methods = append(methods, m)
			}
		}
	}

	methods = append(methods, ssh.PasswordCallback(func() (string, error) {
		return terminal.PromptPassword(fmt.Sprintf("%s's password: ", user))
	}))

	return methods
}

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

func privateKeyMethod(path string) ssh.AuthMethod {
	data, err := os.ReadFile(expandHome(path))
	if err != nil {
		return nil
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(signer)
}

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
