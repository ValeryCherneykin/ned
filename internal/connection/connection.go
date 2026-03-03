// Package connection manages the SSH connection lifecycle and SFTP client setup.
// It owns nothing beyond the dial — callers are responsible for closing
// the returned clients via the provided Closer.
package connection

import (
	"fmt"
	"io"
	"os"

	"github.com/ValeryCherneykin/ned/internal/auth"
	"github.com/ValeryCherneykin/ned/internal/target"
	"github.com/ValeryCherneykin/ned/internal/terminal"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Session holds an active SSH connection and its SFTP client.
// Always call Close when done to release both.
type Session struct {
	SSH  *ssh.Client
	SFTP *sftp.Client
}

// Close shuts down the SFTP client and the underlying SSH connection.
// It returns the first non-nil error encountered.
func (s *Session) Close() error {
	var sftpErr, sshErr error

	if s.SFTP != nil {
		sftpErr = s.SFTP.Close()
	}

	if s.SSH != nil {
		sshErr = s.SSH.Close()
	}

	if sftpErr != nil {
		return sftpErr
	}

	return sshErr
}

// Open dials SSH to the target and initialises an SFTP client on top.
// The caller must call Session.Close() when done.
func Open(t target.Target) (*Session, error) {
	sshClient, err := dialSSH(t)
	if err != nil {
		return nil, fmt.Errorf("SSH dial: %w", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("SFTP init: %w", err)
	}

	return &Session{
		SSH:  sshClient,
		SFTP: sftpClient,
	}, nil
}

// dialSSH opens a raw SSH connection using the auth chain from package auth.
func dialSSH(t target.Target) (*ssh.Client, error) {
	hostKeyCallback, err := buildHostKeyCallback()
	if err != nil {
		// Act_1: warn and fall back. Act_2 will harden this into a hard error.
		terminal.Warn("known_hosts unavailable (%v), skipping host key check", err)
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // act_1 fallback
	}

	cfg := &ssh.ClientConfig{
		User:            t.User,
		Auth:            auth.Methods(t.User),
		HostKeyCallback: hostKeyCallback,
	}

	client, err := ssh.Dial("tcp", t.Addr(), cfg)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", t.Addr(), err)
	}

	return client, nil
}

// buildHostKeyCallback loads ~/.ssh/known_hosts.
// Returns an error if the file is missing or cannot be parsed.
func buildHostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}

	knownHostsPath := home + "/.ssh/known_hosts"

	cb, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", knownHostsPath, err)
	}

	return cb, nil
}

// Ensure Session satisfies io.Closer at compile time.
var _ io.Closer = (*Session)(nil)
