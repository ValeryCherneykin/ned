package transfer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ValeryCherneykin/ned/internal/backend"
)

// Download fetches remotePath via b into a local temp file.
// Returns (tempPath, isNew, error). Caller must os.Remove(tempPath) when done.
func Download(b backend.Backend, remotePath string) (tempPath string, isNew bool, err error) {
	prefix := "ned-" + filepath.Base(remotePath) + "-"

	tmp, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", false, fmt.Errorf("create temp file: %w", err)
	}

	defer func() {
		if closeErr := tmp.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close temp: %w", closeErr)
		}
	}()

	remote, openErr := b.ReadFile(remotePath)
	if openErr != nil {
		if isNotExist(openErr) {
			return tmp.Name(), true, nil
		}
		_ = os.Remove(tmp.Name())
		return "", false, fmt.Errorf("read remote %s: %w", remotePath, openErr)
	}
	defer func() {
		if closeErr := remote.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close remote: %w", closeErr)
		}
	}()

	if _, err = io.Copy(tmp, remote); err != nil {
		_ = os.Remove(tmp.Name())
		return "", false, fmt.Errorf("download %s: %w", remotePath, err)
	}

	return tmp.Name(), false, nil
}

// Upload writes localPath back to remotePath via b.
func Upload(b backend.Backend, localPath, remotePath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local %s: %w", localPath, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close local file: %w", closeErr)
		}
	}()

	if err = b.WriteFile(remotePath, f); err != nil {
		return fmt.Errorf("write remote %s: %w", remotePath, err)
	}
	return nil
}

// strings.Contains is correct, tested, and zero-alloc.
func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) ||
		strings.Contains(err.Error(), "does not exist") ||
		strings.Contains(err.Error(), "no such file")
}
