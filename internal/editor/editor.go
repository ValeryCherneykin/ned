// Package editor resolves the user's preferred local editor and opens a file in it.
// It knows nothing about SSH, SFTP, or remote paths.
package editor

import (
	"fmt"
	"os"
	"os/exec"
)

// preferenceChain is the ordered list of editor candidates tried when $EDITOR is not set.
var preferenceChain = []string{"nvim", "vim", "nano", "vi"}

// Open opens the file at path in the resolved editor and blocks until the editor exits.
// stdin, stdout, and stderr are connected to the calling process so the editor renders correctly.
func Open(path string) error {
	e := resolve()

	cmd := exec.Command(e, path) //nolint:gosec
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %q: %w", e, err)
	}

	return nil
}

// Resolved returns the editor binary that would be used without opening anything.
// Useful for printing status messages before calling Open.
func Resolved() string {
	return resolve()
}

// resolve finds the best available editor.
// Priority: $EDITOR → nvim → vim → nano → vi.
// Always returns a non-empty string — falls back to "vi" as a last resort.
func resolve() string {
	if e := os.Getenv("EDITOR"); e != "" {
		if path, err := exec.LookPath(e); err == nil {
			return path
		}
		// $EDITOR is set but binary not found — return it anyway so the OS
		// gives a descriptive "not found" error rather than silently falling back.
		return e
	}

	for _, candidate := range preferenceChain {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}

	// Nothing found — return "vi" and let the OS produce the error message.
	return "vi"
}
