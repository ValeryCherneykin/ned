package backend

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/pkg/sftp"
)

// SSHBackend implements Backend over an SFTP connection.
type SSHBackend struct {
	client *sftp.Client
}

// NewSSH wraps an existing sftp.Client as a Backend.
func NewSSH(c *sftp.Client) *SSHBackend {
	return &SSHBackend{client: c}
}

func (b *SSHBackend) ReadFile(path string) (io.ReadCloser, error) {
	f, err := b.client.Open(path)
	if err != nil {
		return nil, fmt.Errorf("sftp open %s: %w", path, err)
	}
	return f, nil
}

func (b *SSHBackend) WriteFile(path string, r io.Reader) error {
	if err := b.MkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := b.client.Create(path)
	if err != nil {
		return fmt.Errorf("sftp create %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close remote file: %w", closeErr)
		}
	}()

	if _, err = io.Copy(f, r); err != nil {
		return fmt.Errorf("sftp write %s: %w", path, err)
	}
	return nil
}

func (b *SSHBackend) MkdirAll(path string) error {
	if err := b.client.MkdirAll(path); err != nil {
		return fmt.Errorf("sftp mkdir -p %s: %w", path, err)
	}
	return nil
}

func (b *SSHBackend) Close() error { return b.client.Close() }
