// Package backend defines the storage backend interface and its implementations.
// Adding a new backend (k8s exec, etc.) requires zero changes to existing code.
package backend

import "io"

// Backend abstracts file read/write operations over any remote transport.
type Backend interface {
	// ReadFile opens the remote file for reading.
	// Returns os.ErrNotExist if the file does not exist.
	ReadFile(path string) (io.ReadCloser, error)

	// WriteFile writes r to the remote path, creating parent dirs if needed.
	WriteFile(path string, r io.Reader) error

	// MkdirAll creates the directory path and all parents on the remote.
	MkdirAll(path string) error

	// Close releases underlying resources.
	Close() error
}
