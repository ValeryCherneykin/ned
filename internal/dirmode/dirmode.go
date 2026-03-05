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
	"github.com/ValeryCherneykin/ned/internal/ignore"
	"github.com/ValeryCherneykin/ned/internal/terminal"
	"github.com/ValeryCherneykin/ned/internal/transfer"
)

const (
	pollInterval  = 500 * time.Millisecond
	nedIgnoreFile = ".nedignore"
)

// FileInfo tracks a file in the local tmpdir.
type FileInfo struct {
	ModTime time.Time
}

// Snapshot records the mtime of every file in localDir recursively.
// Ignores paths matched by matcher — pass nil to include everything.
func Snapshot(localDir string, m *ignore.Matcher) (map[string]FileInfo, error) {
	snap := make(map[string]FileInfo)

	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(localDir, path)
		if relErr != nil {
			return relErr
		}

		if rel == "." {
			return nil
		}

		if m.Match(rel, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		if info.IsDir() {
			return nil
		}

		snap[rel] = FileInfo{ModTime: info.ModTime()}

		return nil
	})

	return snap, err
}

// Download recursively downloads remotePath into localDir via b.
// Reads .nedignore from the remote directory if present and skips matched paths.
func Download(b backend.Backend, remotePath, localDir string) error {
	if err := os.MkdirAll(localDir, 0o700); err != nil {
		return fmt.Errorf("create local dir: %w", err)
	}

	m := loadIgnore(b, remotePath)

	return downloadDir(b, remotePath, localDir, m)
}

// loadIgnore reads .nedignore from remotePath if it exists.
// Returns a nil-safe Matcher — safe to use even if file is absent.
func loadIgnore(b backend.Backend, remotePath string) *ignore.Matcher {
	rc, err := b.ReadFile(remotePath + "/" + nedIgnoreFile)
	if err != nil {
		return ignore.ParseString("")
	}

	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		return ignore.ParseString("")
	}

	m := ignore.ParseString(string(data))
	terminal.Status("loaded .nedignore")

	return m
}

func downloadDir(b backend.Backend, remotePath, localDir string, m *ignore.Matcher) error {
	entries, err := b.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("readdir %s: %w", remotePath, err)
	}

	for _, entry := range entries {
		name := filepath.Base(entry.Name)

		// Skip .nedignore itself — no need to edit it locally.
		if name == nedIgnoreFile {
			continue
		}

		if m.Match(name, entry.IsDir) {
			terminal.Status("ignoring %s", name)
			continue
		}

		remoteChild := remotePath + "/" + name
		localChild := filepath.Join(localDir, name)

		if entry.IsDir {
			if err = os.MkdirAll(localChild, 0o700); err != nil {
				return fmt.Errorf("mkdir %s: %w", localChild, err)
			}

			if err := downloadDir(b, remoteChild, localChild, m); err != nil {
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
// Blocks until ctx is cancelled.
func Watch(ctx context.Context, localDir, remoteBase string, b backend.Backend, m *ignore.Matcher) error {
	snap, err := Snapshot(localDir, m)
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
			current, snapErr := Snapshot(localDir, m)
			if snapErr != nil {
				terminal.Warn("snapshot: %v", snapErr)
				continue
			}

			mu.Lock()

			for rel, info := range current {
				prev, existed := snap[rel]

				if existed && info.ModTime.Equal(prev.ModTime) {
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

// UploadChanged uploads only files that changed between before and after snapshots.
// Replaces the old UploadAll to avoid redundant re-upload of unchanged files.
func UploadChanged(b backend.Backend, localDir, remoteBase string, before, after map[string]FileInfo) error {
	for rel, info := range after {
		prev, existed := before[rel]

		if existed && info.ModTime.Equal(prev.ModTime) {
			continue
		}

		localPath := filepath.Join(localDir, rel)
		remotePath := remoteBase + "/" + filepath.ToSlash(rel)

		if err := transfer.Upload(b, localPath, remotePath); err != nil {
			return fmt.Errorf("upload %s: %w", rel, err)
		}
	}

	return nil
}

// RemoteBase normalises a remote directory path.
// Strips trailing slash for consistent use in path joins.
func RemoteBase(path string) string {
	return strings.TrimSuffix(path, "/")
}
