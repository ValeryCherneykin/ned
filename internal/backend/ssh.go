package backend

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/pkg/sftp"
)

// SSHBackend implements Backend over an active SFTP connection.
type SSHBackend struct {
	client *sftp.Client
}

// NewSSH wraps an existing sftp.Client as a Backend.
func NewSSH(c *sftp.Client) *SSHBackend {
	return &SSHBackend{client: c}
}

// ReadFile opens the remote file at path for reading via SFTP.
func (b *SSHBackend) ReadFile(path string) (io.ReadCloser, error) {
	f, err := b.client.Open(path)
	if err != nil {
		return nil, fmt.Errorf("sftp open %s: %w", path, err)
	}

	return f, nil
}

// WriteFile writes r to path on the remote host via SFTP, creating parent dirs as needed.
func (b *SSHBackend) WriteFile(path string, r io.Reader) (err error) {
	if err := b.MkdirAll(filepath.Dir(path)); err != nil {
		return err
	}

	f, err := b.client.Create(path)
	if err != nil {
		return fmt.Errorf("sftp create %s: %w", path, err)
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("sftp close %s: %w", path, closeErr)
		}
	}()

	if _, err = io.Copy(f, r); err != nil {
		return fmt.Errorf("sftp write %s: %w", path, err)
	}

	return nil
}

// MkdirAll creates path and all parent directories on the remote host via SFTP.
func (b *SSHBackend) MkdirAll(path string) error {
	if err := b.client.MkdirAll(path); err != nil {
		return fmt.Errorf("sftp mkdir -p %s: %w", path, err)
	}

	return nil
}

// ReadDir lists the contents of path on the remote host via SFTP (non-recursive).
func (b *SSHBackend) ReadDir(path string) ([]Entry, error) {
	infos, err := b.client.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("sftp readdir %s: %w", path, err)
	}

	entries := make([]Entry, 0, len(infos))

	for _, info := range infos {
		entries = append(entries, Entry{
			Name:  info.Name(),
			IsDir: info.IsDir(),
			Size:  info.Size(),
		})
	}

	return entries, nil
}

// DeleteFile removes a single file from the remote host via SFTP.
func (b *SSHBackend) DeleteFile(path string) error {
	if err := b.client.Remove(path); err != nil {
		return fmt.Errorf("sftp remove %s: %w", path, err)
	}

	return nil
}

// Close releases the underlying SFTP client connection.
func (b *SSHBackend) Close() error { return b.client.Close() }
