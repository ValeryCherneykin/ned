// Package mock provides an in-memory Backend implementation for testing.
// No real SSH or Docker is needed — all operations work against a map[string][]byte.
package mock

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Backend is a thread-safe in-memory implementation of backend.Backend.
// Use New() to create one pre-seeded with files.
type Backend struct {
	mu    sync.RWMutex
	files map[string][]byte

	// MkdirAllCalled tracks which paths were passed to MkdirAll.
	MkdirAllCalled []string

	// Closed reports whether Close was called.
	Closed bool

	// WriteErr forces WriteFile to return this error (for error path tests).
	WriteErr error

	// ReadErr forces ReadFile to return this error (for error path tests).
	ReadErr error
}

// New creates a Backend pre-seeded with the given files.
// Keys are remote paths, values are file contents.
func New(files map[string][]byte) *Backend {
	if files == nil {
		files = make(map[string][]byte)
	}

	return &Backend{files: files}
}

// ReadFile returns the contents of the file at path.
// Returns os.ErrNotExist if the file is not in the map.
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

	// Return a copy so callers cannot mutate the internal map.
	return io.NopCloser(bytes.NewReader(append([]byte(nil), data...))), nil
}

// WriteFile stores r under path, replacing any existing contents.
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

// MkdirAll records the call and is otherwise a no-op.
func (b *Backend) MkdirAll(path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.MkdirAllCalled = append(b.MkdirAllCalled, path)

	return nil
}

// Close marks the backend as closed.
func (b *Backend) Close() error {
	b.Closed = true
	return nil
}

// Get returns the current contents of path, or nil if not found.
// Helper for assertions in tests.
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

// FileCount returns the number of files stored in the backend.
func (b *Backend) FileCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.files)
}

// HasMkdirAll reports whether MkdirAll was called with the given path.
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
