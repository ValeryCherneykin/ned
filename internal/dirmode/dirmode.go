// Package dirmode implements directory edit mode for ned.
// It downloads a remote directory, opens nvim on it, watches for changes,
// uploads modified/new files, and optionally handles deleted files.
package dirmode

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ValeryCherneykin/ned/internal/backend"
	"github.com/ValeryCherneykin/ned/internal/terminal"
	"github.com/ValeryCherneykin/ned/internal/transfer"
)

const pollInterval = 500 * time.Millisecond

// FileInfo tracks a file in the local tmpdir.
type FileInfo struct {
	ModTime time.Time
}

// Snapshot records the mtime of every file in localDir recursively.
// Used to detect new, changed, and deleted files after editing.
func Snapshot(localDir string) (map[string]FileInfo, error) {
	snap := make(map[string]FileInfo)

	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		rel, relErr := filepath.Rel(localDir, path)
		if relErr != nil {
			return relErr
		}

		snap[rel] = FileInfo{ModTime: info.ModTime()}

		return nil
	})

	return snap, err
}

// Download recursively downloads remotePath into localDir via b.
// Creates localDir if it does not exist.
func Download(b backend.Backend, remotePath, localDir string) error {
	if err := os.MkdirAll(localDir, 0o700); err != nil {
		return fmt.Errorf("create local dir: %w", err)
	}

	return downloadDir(b, remotePath, localDir)
}

func downloadDir(b backend.Backend, remotePath, localDir string) error {
	entries, err := b.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("readdir %s: %w", remotePath, err)
	}

	for _, entry := range entries {
		name := filepath.Base(entry.Name)
		remoteChild := remotePath + "/" + name
		localChild := filepath.Join(localDir, name)

		if entry.IsDir {
			if err = os.MkdirAll(localChild, 0o700); err != nil {
				return fmt.Errorf("mkdir %s: %w", localChild, err)
			}
			if err := downloadDir(b, remoteChild, localChild); err != nil {
				return err
			}
			continue
		}

		rc, err := b.ReadFile(remoteChild)
		if err != nil {
			return fmt.Errorf("read %s: %w", remoteChild, err)
		}

		f, err := os.Create(localChild) //nolint:gosec
		if err != nil {
			_ = rc.Close()
			return fmt.Errorf("create %s: %w", localChild, err)
		}

		_, copyErr := io.Copy(f, rc)
		_ = rc.Close()
		_ = f.Close()

		if copyErr != nil {
			return fmt.Errorf("copy %s: %w", remoteChild, copyErr)
		}
	}

	return nil
}

// Watch polls localDir every 500ms and uploads changed/new files to remoteBase via b.
// Blocks until ctx is cancelled. Thread-safe — runs in a goroutine.
func Watch(ctx context.Context, localDir, remoteBase string, b backend.Backend) error {
	snap, err := Snapshot(localDir)
	if err != nil {
		return fmt.Errorf("initial snapshot: %w", err)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var mu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			current, snapErr := Snapshot(localDir)
			if snapErr != nil {
				terminal.Warn("snapshot: %v", snapErr)
				continue
			}

			mu.Lock()

			for rel, info := range current {
				prev, existed := snap[rel]

				changed := !existed || !info.ModTime.Equal(prev.ModTime)
				if !changed {
					continue
				}

				localPath := filepath.Join(localDir, rel)
				remotePath := remoteBase + "/" + filepath.ToSlash(rel)

				if uploadErr := transfer.Upload(b, localPath, remotePath); uploadErr != nil {
					terminal.Warn("upload %s: %v", rel, uploadErr)
				} else {
					if existed {
						terminal.Status("↑ changed %s", rel)
					} else {
						terminal.Status("↑ new     %s", rel)
					}
				}
			}

			snap = current
			mu.Unlock()
		}
	}
}

// CollectDeleted returns relative paths that existed in before but not in after.
func CollectDeleted(before, after map[string]FileInfo) []string {
	var deleted []string

	for rel := range before {
		if _, ok := after[rel]; !ok {
			deleted = append(deleted, rel)
		}
	}

	return deleted
}

// UploadAll uploads every file in localDir to remoteBase.
// Used for final sync on editor exit.
func UploadAll(b backend.Backend, localDir, remoteBase string) error {
	return filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}

		remotePath := remoteBase + "/" + filepath.ToSlash(rel)

		return transfer.Upload(b, path, remotePath)
	})
}

// RemoteBase normalises a remote directory path.
// Strips trailing slash for consistent use in path joins.
func RemoteBase(path string) string {
	return strings.TrimSuffix(path, "/")
}
