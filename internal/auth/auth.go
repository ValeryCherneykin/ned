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
