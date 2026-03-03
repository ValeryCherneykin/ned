package editor_test

import (
	"os"
	"testing"

	"github.com/ValeryCherneykin/ned/internal/editor"
)

func TestResolved_UsesEditorEnv(t *testing.T) {
	t.Setenv("EDITOR", "nano")

	got := editor.Resolved()
	if got == "" {
		t.Error("Resolved() returned empty string")
	}
}

func TestResolved_FallsBackToChain(t *testing.T) {
	t.Setenv("EDITOR", "")

	got := editor.Resolved()
	if got == "" {
		t.Error("Resolved() fallback returned empty string")
	}
}

func TestOpen_NonExistentEditor(t *testing.T) {
	t.Setenv("EDITOR", "this-editor-definitely-does-not-exist-12345")

	tmp, err := os.CreateTemp("", "ned-editor-test-")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	defer os.Remove(tmp.Name())
	tmp.Close()

	// Should return an error — the binary doesn't exist.
	if err = editor.Open(tmp.Name()); err == nil {
		t.Error("Open() expected error for non-existent editor, got nil")
	}
}
