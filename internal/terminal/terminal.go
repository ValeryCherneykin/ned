package terminal

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func Status(format string, args ...any)  { fmt.Printf("→ "+format+"\n", args...) }
func Success(format string, args ...any) { fmt.Printf("✓ "+format+"\n", args...) }
func Warn(format string, args ...any)    { fmt.Fprintf(os.Stderr, "⚠  "+format+"\n", args...) }

func Fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
	os.Exit(1)
}

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

func PrintUsage() {
	fmt.Fprintln(os.Stderr, `ned — open a remote file in your local editor over SSH or Docker

usage:
  ned [flags] [user@]host[:port]:/remote/path
  ned [flags] docker://container:/remote/path

flags:
  -i <identity>   path to SSH private key
  -p <port>       SSH port override

examples:
  ned root@192.168.1.10:/etc/nginx/nginx.conf
  ned -i ~/.ssh/prod deploy@prod.example.com:/app/.env
  ned docker://my-container:/app/config.json
  ned prod:/etc/.env`)
}
