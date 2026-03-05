package backend

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DockerBackend implements Backend via `docker exec`.
// No SSH, no ports, no keys — just the container name or ID.
type DockerBackend struct {
	container string
}

// NewDocker creates a DockerBackend targeting the given container name or ID.
func NewDocker(container string) *DockerBackend {
	return &DockerBackend{container: container}
}

// ReadFile reads the file at path from the container via `docker exec cat`.
func (b *DockerBackend) ReadFile(path string) (io.ReadCloser, error) {
	out, err := exec.Command("docker", "exec", b.container, "cat", path).Output() //nolint:gosec
	if err != nil {
		if isDockerNotExist(err) {
			return nil, fmt.Errorf("cat %s: %w", path, fmt.Errorf("does not exist"))
		}

		return nil, fmt.Errorf("docker exec cat %s: %w", path, err)
	}

	return io.NopCloser(bytes.NewReader(out)), nil
}

// WriteFile writes r to path inside the container via `docker exec tee`.
// Parent directories are created automatically via MkdirAll.
func (b *DockerBackend) WriteFile(path string, r io.Reader) error {
	if err := b.MkdirAll(filepath.Dir(path)); err != nil {
		return err
	}

	cmd := exec.Command("docker", "exec", "-i", b.container, "tee", path) //nolint:gosec
	cmd.Stdin = r

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker exec tee %s: %w\n%s", path, err, out)
	}

	return nil
}

// MkdirAll creates path and all parents inside the container.
func (b *DockerBackend) MkdirAll(path string) error {
	out, err := exec.Command("docker", "exec", b.container, "mkdir", "-p", path).CombinedOutput() //nolint:gosec
	if err != nil {
		return fmt.Errorf("docker exec mkdir -p %s: %w\n%s", path, err, out)
	}

	return nil
}

// ReadDir lists the contents of path inside the container (non-recursive).
// Uses `ls -1A --file-type` for a simple listing.
// ReadDir lists the contents of path inside the container (non-recursive).
func (b *DockerBackend) ReadDir(path string) ([]Entry, error) {
	out, err := exec.Command( //nolint:gosec
		"docker", "exec", b.container,
		"ls", "-1p", path,
	).Output()
	if err != nil {
		if isDockerNotExist(err) {
			return nil, fmt.Errorf("readdir %s: %w", path, os.ErrNotExist)
		}
		return nil, fmt.Errorf("docker exec ls %s: %w", path, err)
	}

	entries := make([]Entry, 0, 8)

	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		isDir := strings.HasSuffix(name, "/")
		name = strings.TrimSuffix(name, "/")

		entries = append(entries, Entry{
			Name:  name,
			IsDir: isDir,
		})
	}

	return entries, nil
}

// DeleteFile removes a single file inside the container via `docker exec rm`.
func (b *DockerBackend) DeleteFile(path string) error {
	out, err := exec.Command("docker", "exec", b.container, "rm", "-f", path).CombinedOutput() //nolint:gosec
	if err != nil {
		return fmt.Errorf("docker exec rm %s: %w\n%s", path, err, out)
	}

	return nil
}

// Close is a no-op — docker exec commands are stateless.
func (b *DockerBackend) Close() error { return nil }

// isDockerNotExist reports whether err indicates a missing file inside the container.
func isDockerNotExist(err error) bool {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return bytes.Contains(ee.Stderr, []byte("No such file"))
	}

	return false
}

// IsDir reports whether path is a directory inside the container.
func (b *DockerBackend) IsDir(path string) bool {
	err := exec.Command("docker", "exec", b.container, "test", "-d", path).Run() //nolint:gosec
	return err == nil
}
