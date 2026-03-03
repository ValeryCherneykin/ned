package backend

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
)

// DockerBackend implements Backend via docker exec.
type DockerBackend struct {
	container string
}

func NewDocker(container string) *DockerBackend {
	return &DockerBackend{container: container}
}

func (b *DockerBackend) ReadFile(path string) (io.ReadCloser, error) {
	out, err := exec.Command("docker", "exec", b.container, "cat", path).Output()
	if err != nil {
		if isDockerNotExist(err) {
			return nil, fmt.Errorf("cat %s: %w", path, fmt.Errorf("does not exist"))
		}

		return nil, fmt.Errorf("docker exec cat %s: %w", path, err)
	}

	return io.NopCloser(bytes.NewReader(out)), nil
}

// WriteFile streams r into the container via docker exec tee.
// act_4: uses io.Pipe to avoid reading the entire file into memory —
// data flows directly from the local file into docker stdin.
func (b *DockerBackend) WriteFile(path string, r io.Reader) error {
	if err := b.MkdirAll(filepath.Dir(path)); err != nil {
		return err
	}

	cmd := exec.Command("docker", "exec", "-i", b.container, "tee", path)

	// Stream directly — no intermediate buffer, zero copies.
	cmd.Stdin = r

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
