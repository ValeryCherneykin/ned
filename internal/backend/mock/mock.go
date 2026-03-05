// Package mock provides an in-memory Backend implementation for testing.
package mock

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ValeryCherneykin/ned/internal/backend"
)

// Backend is a thread-safe in-memory implementation of backend.Backend.
type Backend struct {
	WriteErr       error
	ReadErr        error
	DeleteErr      error
	Closed         bool
	MkdirAllCalled []string

	mu    sync.RWMutex
	files map[string][]byte
}

// New creates a Backend pre-seeded with the given files.
func New(files map[string][]byte) *Backend {
	if files == nil {
		files = make(map[string][]byte)
	}

	return &Backend{files: files}
}

// ReadFile returns the contents of the file at path.
func (b *Backend) ReadFile(path string) (io.ReadCloser, error) {
	if b.ReadErr != nil {
		return nil, b.ReadErr
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	data, ok := b.files[path]
	if !ok {
		return nil, fmt.Errorf("open %s: %w", path, os.ErrNotExist)
	}

	return io.NopCloser(bytes.NewReader(append([]byte(nil), data...))), nil
}

// WriteFile stores r under path.
func (b *Backend) WriteFile(path string, r io.Reader) error {
	if b.WriteErr != nil {
		return b.WriteErr
	}

	var buf bytes.Buffer

	if _, err := io.Copy(&buf, r); err != nil {
		return fmt.Errorf("read: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.files[path] = buf.Bytes()

	return nil
}

// MkdirAll records the call and is a no-op.
func (b *Backend) MkdirAll(path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.MkdirAllCalled = append(b.MkdirAllCalled, path)

	return nil
}

// ReadDir lists all entries whose path starts with path/.
func (b *Backend) ReadDir(path string) ([]backend.Entry, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	prefix := strings.TrimSuffix(path, "/") + "/"
	seen := make(map[string]bool)
	entries := make([]backend.Entry, 0, len(b.files))

	for key := range b.files {
		if !strings.HasPrefix(key, prefix) {
			continue
		}

		rest := strings.TrimPrefix(key, prefix)
		parts := strings.SplitN(rest, "/", 2)
		name := parts[0]

		if seen[name] {
			continue
		}

		seen[name] = true
		isDir := len(parts) > 1

		entries = append(entries, backend.Entry{
			Name:  name,
			IsDir: isDir,
			Size:  int64(len(b.files[key])),
		})
	}

	return entries, nil
}

// DeleteFile removes path from the in-memory store.
func (b *Backend) DeleteFile(path string) error {
	if b.DeleteErr != nil {
		return b.DeleteErr
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.files[path]; !ok {
		return fmt.Errorf("remove %s: %w", path, os.ErrNotExist)
	}

	delete(b.files, path)

	return nil
}

// Close marks the backend as closed.
func (b *Backend) Close() error {
	b.Closed = true
	return nil
}

// Get returns the current contents of path, or nil if not found.
func (b *Backend) Get(path string) []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.files[path]
}

// Has reports whether path exists in the backend.
func (b *Backend) Has(path string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	_, ok := b.files[path]

	return ok
}

// FileCount returns the number of files stored.
func (b *Backend) FileCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.files)
}

// HasMkdirAll reports whether MkdirAll was called with path.
func (b *Backend) HasMkdirAll(path string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, p := range b.MkdirAllCalled {
		if p == path {
			return true
		}
	}

	return false
}

// HasMkdirAllPrefix reports whether any MkdirAll call started with prefix.
func (b *Backend) HasMkdirAllPrefix(prefix string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, p := range b.MkdirAllCalled {
		if strings.HasPrefix(p, prefix) || p == filepath.Dir(prefix) {
			return true
		}
	}

	return false
}
