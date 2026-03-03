// Package transfer handles downloading remote files to local temp files
// and uploading them back after editing.
//
// act_4 optimizations:
//   - io.CopyBuffer with sync.Pool-backed buffers eliminates the default
//     32KB heap allocation inside every io.Copy call.
package transfer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ValeryCherneykin/ned/internal/backend"
)

const copyBufSize = 32 * 1024

// bufPool holds reusable []byte for io.CopyBuffer.
// Hot path: Download → edit → Upload reuses same buffer, zero extra allocs.
var bufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, copyBufSize)
		return &buf
	},
}

func getBuf() *[]byte  { return bufPool.Get().(*[]byte) } //nolint:forcetypeassert
func putBuf(b *[]byte) { bufPool.Put(b) }

// Download fetches remotePath via b into a new local temp file.
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

	buf := getBuf()
	defer putBuf(buf)

	if _, err = io.CopyBuffer(tmp, remote, *buf); err != nil {
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
		strings.Contains(err.Error(), "does not exist") ||
		strings.Contains(err.Error(), "no such file")
}
