package backend

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
)

// DockerBackend implements Backend via `docker exec`.
// No SSH, no ports, no keys — just the container name.
type DockerBackend struct {
	container string
}

// NewDocker creates a DockerBackend for the given container name or ID.
func NewDocker(container string) *DockerBackend {
	return &DockerBackend{container: container}
}

func (b *DockerBackend) ReadFile(path string) (io.ReadCloser, error) {
	out, err := exec.Command("docker", "exec", b.container, "cat", path).Output()
	if err != nil {
		// Map "No such file" to a recognisable error.
		if isDockerNotExist(err) {
			return nil, fmt.Errorf("cat %s: %w", path, fmt.Errorf("does not exist"))
		}
		return nil, fmt.Errorf("docker exec cat %s: %w", path, err)
	}
	return io.NopCloser(bytes.NewReader(out)), nil
}

func (b *DockerBackend) WriteFile(path string, r io.Reader) error {
	if err := b.MkdirAll(filepath.Dir(path)); err != nil {
		return err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read local file: %w", err)
	}

	// Pipe content into the container via tee.
	cmd := exec.Command("docker", "exec", "-i", b.container, "tee", path)
	cmd.Stdin = bytes.NewReader(data)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker exec tee %s: %w\n%s", path, err, out)
	}
	return nil
}

func (b *DockerBackend) MkdirAll(path string) error {
	out, err := exec.Command("docker", "exec", b.container, "mkdir", "-p", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker exec mkdir -p %s: %w\n%s", path, err, out)
	}
	return nil
}

func (b *DockerBackend) Close() error { return nil }

func isDockerNotExist(err error) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		return bytes.Contains(ee.Stderr, []byte("No such file"))
	}
	return false
}
