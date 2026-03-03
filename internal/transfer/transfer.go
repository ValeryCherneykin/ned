// Package transfer handles downloading remote files to local temp files
// and uploading them back after editing.
//
// It knows nothing about SSH or editors — it only speaks sftp.Client
// and file paths.
package transfer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
)

// Download fetches remotePath from the SFTP client into a new local temp file.
// If the remote file does not exist, an empty temp file is created and
// (tempPath, true, nil) is returned — the boolean signals a new file.
//
// The caller is responsible for removing the temp file when done.
func Download(c *sftp.Client, remotePath string) (tempPath string, isNew bool, err error) {
	// Name the temp file after the remote basename for a better editor title bar.
	prefix := "ned-" + filepath.Base(remotePath) + "-"

	tmp, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", false, fmt.Errorf("create temp file: %w", err)
	}

	defer func() {
		// Always close the temp file; surface the first error.
		if closeErr := tmp.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close temp file: %w", closeErr)
		}
	}()

	remote, openErr := c.Open(remotePath)
	if openErr != nil {
		if isNotExist(openErr) {
			// File is new — return the empty temp file.
			return tmp.Name(), true, nil
		}

		_ = os.Remove(tmp.Name())
		return "", false, fmt.Errorf("open remote %s: %w", remotePath, openErr)
	}

	defer remote.Close()

	if _, err = io.Copy(tmp, remote); err != nil {
		_ = os.Remove(tmp.Name())
		return "", false, fmt.Errorf("download %s: %w", remotePath, err)
	}

	return tmp.Name(), false, nil
}

// Upload writes the local file at localPath to remotePath on the SFTP server.
// It creates all necessary parent directories before writing.
func Upload(c *sftp.Client, localPath, remotePath string) error {
	// Ensure parent directories exist on the remote.
	remoteDir := filepath.Dir(remotePath)

	if err := c.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("mkdir -p %s: %w", remoteDir, err)
	}

	local, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file %s: %w", localPath, err)
	}

	defer local.Close()

	remote, err := c.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}

	defer remote.Close()

	if _, err = io.Copy(remote, local); err != nil {
		return fmt.Errorf("upload to %s: %w", remotePath, err)
	}

	return nil
}

// isNotExist reports whether an SFTP error means the file was not found.
func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) ||
		strings.Contains(err.Error(), "does not exist") ||
		strings.Contains(err.Error(), "no such file")
}
