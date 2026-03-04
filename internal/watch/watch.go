// Package watch implements live upload on local file save.
// It watches the parent directory of the temp file instead of the file itself,
// because editors like vim/neovim write atomically (write to temp → rename),
// which causes fsnotify to lose the file handle if we watch the file directly.
// Package watch implements live upload on local file save.
// Watches the real (symlink-resolved) path of the parent directory
// so FSEvents on macOS works correctly (/tmp is a symlink to /private/tmp).
// Package watch implements live upload on local file save.
// Uses mtime polling (500ms interval) instead of fsnotify — more reliable
// across macOS/Linux and avoids FSEvents symlink issues with /tmp.
package watch

import (
	"context"
	"os"
	"time"

	"github.com/ValeryCherneykin/ned/internal/backend"
	"github.com/ValeryCherneykin/ned/internal/terminal"
	"github.com/ValeryCherneykin/ned/internal/transfer"
)

const pollInterval = 500 * time.Millisecond

// Watch polls localPath every 500ms and uploads to remotePath via b
// whenever the file's mtime changes. Blocks until ctx is cancelled.
func Watch(ctx context.Context, localPath, remotePath string, b backend.Backend) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	lastMod := info.ModTime()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			info, err = os.Stat(localPath)
			if err != nil {
				// File briefly missing during vim atomic write — ignore.
				continue
			}

			if info.ModTime().Equal(lastMod) {
				continue
			}

			lastMod = info.ModTime()

			if uploadErr := transfer.Upload(b, localPath, remotePath); uploadErr != nil {
				terminal.Warn("watch upload failed: %v", uploadErr)
			} else {
				terminal.Status("↑ saved %s", remotePath)
			}
		}
	}
}
