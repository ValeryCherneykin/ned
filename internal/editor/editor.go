// Package editor resolves the user's preferred local editor and opens a file in it.
// It knows nothing about SSH, SFTP, or file paths beyond what it receives.
package editor

import (
	"fmt"
	"os"
	"os/exec"
)

// preferenceChain is the ordered list of editor candidates when $EDITOR is not set.
var preferenceChain = []string{"nvim", "vim", "nano", "vi"}

// Open opens the file at path in the user's preferred editor and blocks until
// the editor exits. The editor's stdin/stdout/stderr are connected to the
// calling process's terminal so it renders correctly.
func Open(path string) error {
	editor, err := resolve()
	if err != nil {
		return err
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Run(); err != nil {
		return fmt.Errorf("editor %q: %w", editor, err)
	}

	return nil
}

// Resolved returns the editor binary that would be used, without opening anything.
// Useful for status messages before calling Open.
func Resolved() string {
	e, _ := resolve()
	return e
}

// resolve finds the best available editor.
// Priority: $EDITOR env var → preference chain → "vi" as a hard fallback.
func resolve() (string, error) {
	// Respect the user's explicit preference first.
	if e := os.Getenv("EDITOR"); e != "" {
		if path, err := exec.LookPath(e); err == nil {
			return path, nil
		}
		// $EDITOR is set but the binary is not found — still try it and let
		// the OS give a descriptive error rather than silently falling back.
		return e, nil
	}

	for _, candidate := range preferenceChain {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}

	// Nothing found — return "vi" and let the OS error be the message.
	return "vi", nil
}
