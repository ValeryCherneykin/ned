// Package backend defines the storage backend interface and its implementations.
// Adding a new backend (k8s exec, etc.) requires zero changes to existing code.
package backend

import "io"

// Entry represents a single item in a remote directory listing.
type Entry struct {
	// Name is the base name of the file or directory (not the full path).
	Name string
	// IsDir reports whether the entry is a directory.
	IsDir bool
	// Size is the file size in bytes (0 for directories).
	Size int64
}

// Backend abstracts file read/write operations over any remote transport.
type Backend interface {
	// ReadFile opens the remote file for reading.
	// Returns os.ErrNotExist if the file does not exist.
	ReadFile(path string) (io.ReadCloser, error)

	// WriteFile writes r to the remote path, creating parent dirs if needed.
	WriteFile(path string, r io.Reader) error

	// MkdirAll creates the directory path and all parents on the remote.
	MkdirAll(path string) error

	// ReadDir lists the contents of a remote directory (non-recursive).
	// Returns os.ErrNotExist if path does not exist.
	ReadDir(path string) ([]Entry, error)

	// DeleteFile removes a single file from the remote.
	// Returns os.ErrNotExist if the file does not exist.
	DeleteFile(path string) error

	// Close releases underlying resources.
	Close() error
}
