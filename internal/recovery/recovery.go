// Package recovery saves edited files locally when an upload fails,
// preventing data loss on network errors or crashes.
//
// Recovered files are stored at:
//
//	~/.ned/recovery/<basename>_<timestamp>
//
// The user can retry the upload with:
//
//	ned --recover ~/.ned/recovery/<file> user@host:/path
package recovery

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	// DirName is the recovery directory under ~/.ned/.
	DirName = ".ned/recovery"
)

// Save copies src to ~/.ned/recovery/<basename>_<timestamp>.
// Returns the path of the saved file so it can be shown to the user.
func Save(src, remotePath string) (string, error) {
	dir, err := ensureDir()
	if err != nil {
		return "", fmt.Errorf("ensure recovery dir: %w", err)
	}

	base := filepath.Base(remotePath)
	timestamp := time.Now().Format("20060102_150405")
	name := fmt.Sprintf("%s_%s", base, timestamp)
	dst := filepath.Join(dir, name)

	if err = copyFile(src, dst); err != nil {
		return "", fmt.Errorf("save recovery file: %w", err)
	}

	return dst, nil
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}

	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}

	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return nil
}

// ensureDir creates ~/.ned/recovery/ if it does not exist.
func ensureDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}

	dir := filepath.Join(home, DirName)

	if err = os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}

	return dir, nil
}
