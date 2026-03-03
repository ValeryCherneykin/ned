package transfer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	defer remote.Close()

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
	defer f.Close()

	if err = b.WriteFile(remotePath, f); err != nil {
		return fmt.Errorf("write remote %s: %w", remotePath, err)
	}
	return nil
}

func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) ||
		contains(err.Error(), "does not exist") ||
		contains(err.Error(), "no such file")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
