// Package terminal handles all user-facing I/O: status messages, prompts,
// and error formatting. Internal packages must never write to stdout/stderr
// directly — they return errors, and this package handles presentation.
package terminal

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// Status prints a progress message to stdout prefixed with "→".
func Status(format string, args ...any) { fmt.Printf("→ "+format+"\n", args...) }

// Success prints a completion message to stdout prefixed with "✓".
func Success(format string, args ...any) { fmt.Printf("✓ "+format+"\n", args...) }

// Warn prints a non-fatal warning to stderr prefixed with "⚠".
func Warn(format string, args ...any) { fmt.Fprintf(os.Stderr, "⚠  "+format+"\n", args...) }

// Fatal prints an error to stderr and exits with code 1.
// Only cmd/ned should call this — internal packages must return errors instead.
func Fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
	os.Exit(1)
}

// PromptPassword writes prompt and reads a password without echoing characters.
// Uses golang.org/x/term on real terminals, falls back to plain Scanln on
// non-TTY input (pipes, CI, scripted use).
func PromptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	if term.IsTerminal(int(os.Stdin.Fd())) {
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()

		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}

		return string(pw), nil
	}

	var pw string
	if _, err := fmt.Scanln(&pw); err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	return pw, nil
}

// Confirm asks a yes/no question on stdout and reads the answer from stdin.
// Returns defaultYes when the user presses Enter without typing anything.
func Confirm(prompt string, defaultYes bool) bool {
	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}

	fmt.Printf("%s %s: ", prompt, hint)

	var ans string
	if _, err := fmt.Scanln(&ans); err != nil || ans == "" {
		return defaultYes
	}

	switch ans {
	case "y", "Y", "yes":
		return true
	case "n", "N", "no":
		return false
	default:
		return defaultYes
	}
}

// PrintUsage writes the ned usage string to stderr.
func PrintUsage() {
	fmt.Fprintln(os.Stderr, `ned — open a remote file in your local editor over SSH or Docker
usage:
  ned [flags] [user@]host[:port]:/remote/path
  ned [flags] docker://container:/remote/path
flags:
  -i <identity>   path to SSH private key
  -p <port>       SSH port override
  --version       print version and exit
examples:
  ned root@192.168.1.10:/etc/nginx/nginx.conf
  ned -i ~/.ssh/prod deploy@prod.example.com:/app/.env
  ned docker://my-container:/app/config.json
  ned prod:/etc/.env`)
}
