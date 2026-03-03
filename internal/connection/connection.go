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

// Options configures an SSH connection.
type Options struct {
	// IdentityFile overrides the default key search with a specific private key path.
	IdentityFile string
}

// Option is a functional option for connection.Open.
type Option func(*Options)

// WithIdentityFile sets an explicit private key path.
func WithIdentityFile(path string) Option {
	return func(o *Options) { o.IdentityFile = path }
}

// Session holds an active SSH connection and its SFTP client.
type Session struct {
	SSH  *ssh.Client
	SFTP *sftp.Client
}

// Close shuts down SFTP then SSH, returning the first error.
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

// RunCommand executes a shell command on the remote host and returns stdout.
// Used by keygen to install the public key.
func (s *Session) RunCommand(cmd string) (string, error) {
	sess, err := s.SSH.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	out, err := sess.Output(cmd)
	if err != nil {
		return "", fmt.Errorf("run %q: %w", cmd, err)
	}
	return string(out), nil
}

// Open dials SSH to the target and initialises an SFTP client on top.
func Open(t target.Target, opts ...Option) (*Session, error) {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}

	sshClient, err := dialSSH(t, o)
	if err != nil {
		return nil, fmt.Errorf("SSH dial: %w", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("SFTP init: %w", err)
	}

	return &Session{SSH: sshClient, SFTP: sftpClient}, nil
}

func dialSSH(t target.Target, o *Options) (*ssh.Client, error) {
	hostKeyCallback, err := buildHostKeyCallback()
	if err != nil {
		terminal.Warn("known_hosts unavailable (%v), skipping host key check", err)
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec
	}

	cfg := &ssh.ClientConfig{
		User:            t.User,
		Auth:            auth.Methods(t.User, o.IdentityFile),
		HostKeyCallback: hostKeyCallback,
	}

	client, err := ssh.Dial("tcp", t.Addr(), cfg)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", t.Addr(), err)
	}
	return client, nil
}

func buildHostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home: %w", err)
	}
	cb, err := knownhosts.New(home + "/.ssh/known_hosts")
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}
	return cb, nil
}

var _ io.Closer = (*Session)(nil)
