package backend

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
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
// Returns a wrapped os.ErrNotExist error if the file is absent inside the container.
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

	// Stream directly into container stdin — no intermediate buffer needed.
	cmd := exec.Command("docker", "exec", "-i", b.container, "tee", path) //nolint:gosec
	cmd.Stdin = r

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker exec tee %s: %w\n%s", path, err, out)
	}

	return nil
}

// MkdirAll creates path and all parent directories inside the container.
func (b *DockerBackend) MkdirAll(path string) error {
	out, err := exec.Command("docker", "exec", b.container, "mkdir", "-p", path).CombinedOutput() //nolint:gosec
	if err != nil {
		return fmt.Errorf("docker exec mkdir -p %s: %w\n%s", path, err, out)
	}

	return nil
}

// Close is a no-op for DockerBackend — docker exec commands are stateless.
func (b *DockerBackend) Close() error { return nil }

// isDockerNotExist reports whether err indicates a missing file inside the container.
// Uses errors.As to correctly unwrap the ExitError from any wrapping.
func isDockerNotExist(err error) bool {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return bytes.Contains(ee.Stderr, []byte("No such file"))
	}

	return false
}
