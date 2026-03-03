// Package terminal handles all user-facing I/O:
// status messages, prompts, and error formatting.
// Internal packages must never write to stdout/stderr directly —
// they return errors and let this package handle presentation.
package terminal

import (
	"fmt"
	"os"
)

// Status prints a progress line prefixed with "→".
func Status(format string, args ...any) {
	fmt.Printf("→ "+format+"\n", args...)
}

// Success prints a completion line prefixed with "✓".
func Success(format string, args ...any) {
	fmt.Printf("✓ "+format+"\n", args...)
}

// Warn prints a warning to stderr prefixed with "⚠".
func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "⚠  "+format+"\n", args...)
}

// Fatal prints an error to stderr and exits with code 1.
// Only cmd/ned should call this — internal packages must return errors.
func Fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
	os.Exit(1)
}

// PromptPassword writes a prompt and reads a plaintext password from stdin.
// NOTE: In act_2 this will be upgraded to golang.org/x/term for no-echo input.
func PromptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	var password string

	if _, err := fmt.Scanln(&password); err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	return password, nil
}

// PrintUsage writes a short help message to stderr.
func PrintUsage() {
	lines := []string{
		"ned — open a remote file in your local editor over SSH",
		"",
		"usage:",
		"  ned [user@]host[:port]:/remote/path",
		"",
		"examples:",
		"  ned root@192.168.1.10:/etc/nginx/nginx.conf",
		"  ned deploy@prod.example.com:/app/.env",
		"  ned 10.0.0.1:2222:/tmp/test.txt",
	}

	for _, l := range lines {
		fmt.Fprintln(os.Stderr, l)
	}
}
