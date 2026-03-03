// ned — open a remote file in your local editor over SSH/SFTP.
//
// Usage:
//
//	ned [user@]host:/path/to/file
//
// Examples:
//
//	ned root@192.168.1.10:/etc/nginx/nginx.conf
//	ned deploy@prod.example.com:/app/.env
//	ned 192.168.1.10:/tmp/test.txt          (uses current OS user)
package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	// defaultSSHPort is used when no port is specified in the target string.
	defaultSSHPort = "22"

	// defaultUser falls back to the OS user when none is provided in the target.
	defaultUser = ""
)

// target holds the parsed components of the CLI argument.
//
//	[user@]host[:port]:/remote/path
type target struct {
	user       string
	host       string
	port       string
	remotePath string
}

func main() {
	if len(os.Args) != 2 {
		printUsage()
		os.Exit(1)
	}

	t, err := parseTarget(os.Args[1])
	if err != nil {
		fatalf("invalid target %q: %v", os.Args[1], err)
	}

	// Resolve user: CLI arg → $USER env var → "root" as last resort.
	if t.user == "" {
		t.user = currentUser()
	}

	fmt.Printf("→ connecting %s@%s:%s\n", t.user, t.host, t.port)

	client, err := sshDial(t)
	if err != nil {
		fatalf("SSH connection failed: %v", err)
	}
	defer client.Close()

	fmt.Printf("→ connected\n")

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		fatalf("SFTP init failed: %v", err)
	}
	defer sftpClient.Close()

	// Pull remote file into a local temp file.
	tmpFile, err := downloadToTemp(sftpClient, t.remotePath)
	if err != nil {
		fatalf("download failed: %v", err)
	}
	defer os.Remove(tmpFile) // always clean up, even on error

	fmt.Printf("→ %s downloaded to %s\n", t.remotePath, tmpFile)

	// Open the temp file in the user's preferred editor and wait for exit.
	if err = openEditor(tmpFile); err != nil {
		fatalf("editor error: %v", err)
	}

	// Upload the (possibly modified) temp file back to the server.
	fmt.Printf("→ uploading changes to %s\n", t.remotePath)

	if err = uploadFromTemp(sftpClient, tmpFile, t.remotePath); err != nil {
		fatalf("upload failed: %v", err)
	}

	fmt.Printf("✓ saved %s@%s:%s\n", t.user, t.host, t.remotePath)
}

// ─────────────────────────────────────────────
// Argument parsing
// ─────────────────────────────────────────────

// parseTarget splits a string of the form [user@]host[:port]:/path into a target.
// The colon before the path is mandatory and distinguishes the host from the path.
//
//	"root@10.0.0.1:/etc/.env"       → user=root, host=10.0.0.1, port=22, path=/etc/.env
//	"10.0.0.1:2222:/tmp/file.txt"   → user="", host=10.0.0.1, port=2222, path=/tmp/file.txt
//	"10.0.0.1:/tmp/file.txt"        → user="", host=10.0.0.1, port=22, path=/tmp/file.txt
func parseTarget(raw string) (target, error) {
	// Remote path starts after the first slash that follows a colon.
	// We look for ":/", which separates host[:port] from /remote/path.
	colonSlash := strings.Index(raw, ":/")
	if colonSlash == -1 {
		return target{}, errors.New("missing ':/path' separator — expected [user@]host:/path")
	}

	hostPart := raw[:colonSlash]
	remotePath := raw[colonSlash+1:] // includes leading "/"

	if remotePath == "" || remotePath == "/" {
		return target{}, errors.New("remote path cannot be empty or just '/'")
	}

	// Split optional user@ from hostPart.
	var user, hostPort string

	if at := strings.LastIndex(hostPart, "@"); at != -1 {
		user = hostPart[:at]
		hostPort = hostPart[at+1:]
	} else {
		hostPort = hostPart
	}

	// Split optional :port from host.
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		// No port provided — net.SplitHostPort fails; treat the whole thing as host.
		host = hostPort
		port = defaultSSHPort
	}

	if host == "" {
		return target{}, errors.New("host cannot be empty")
	}

	if user != "" && strings.ContainsAny(user, " \t") {
		return target{}, errors.New("user contains invalid characters")
	}

	return target{
		user:       user,
		host:       host,
		port:       port,
		remotePath: remotePath,
	}, nil
}

// ─────────────────────────────────────────────
// SSH connection
// ─────────────────────────────────────────────

// sshDial opens an SSH connection using a chain of auth methods:
//  1. SSH agent (if $SSH_AUTH_SOCK is set)
//  2. Default private key files: ~/.ssh/id_ed25519, id_rsa, id_ecdsa
//  3. Interactive password prompt as a last resort
func sshDial(t target) (*ssh.Client, error) {
	authMethods := buildAuthMethods()

	hostKeyCallback, err := buildHostKeyCallback()
	if err != nil {
		// If known_hosts is missing/broken we fall back to InsecureIgnoreHostKey.
		// In act_2 this will become a hard error with a proper UX.
		fmt.Fprintf(os.Stderr, "⚠  known_hosts unavailable (%v), skipping host key check\n", err)

		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // act_0: temporary fallback
	}

	cfg := &ssh.ClientConfig{
		User:            t.user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	addr := net.JoinHostPort(t.host, t.port)

	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	return client, nil
}

// buildAuthMethods assembles SSH auth methods in priority order.
func buildAuthMethods() []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	// 1. SSH agent — zero friction if the user's keys are loaded.
	if agentMethod := sshAgentAuth(); agentMethod != nil {
		methods = append(methods, agentMethod)
	}

	// 2. Common private key files.
	for _, keyPath := range defaultKeyPaths() {
		if method := privateKeyAuth(keyPath); method != nil {
			methods = append(methods, method)
		}
	}

	// 3. Password prompt — last resort.
	methods = append(methods, ssh.PasswordCallback(passwordPrompt))

	return methods
}

// sshAgentAuth returns an auth method backed by the local SSH agent,
// or nil if the agent socket is not available.
func sshAgentAuth() ssh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}

	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}

	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers)
}

// privateKeyAuth loads a single private key file and returns a PublicKeys auth method.
// Returns nil if the file doesn't exist or cannot be parsed.
func privateKeyAuth(path string) ssh.AuthMethod {
	expanded := expandHome(path)

	data, err := os.ReadFile(expanded)
	if err != nil {
		return nil // file simply doesn't exist
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		// Key might be passphrase-protected; skip silently in act_0.
		return nil
	}

	return ssh.PublicKeys(signer)
}

// defaultKeyPaths returns the standard private key locations to try.
func defaultKeyPaths() []string {
	return []string{
		"~/.ssh/id_ed25519",
		"~/.ssh/id_rsa",
		"~/.ssh/id_ecdsa",
	}
}

// buildHostKeyCallback loads ~/.ssh/known_hosts for host verification.
func buildHostKeyCallback() (ssh.HostKeyCallback, error) {
	knownHostsFile := expandHome("~/.ssh/known_hosts")

	cb, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts %s: %w", knownHostsFile, err)
	}

	return cb, nil
}

// passwordPrompt is called by ssh.PasswordCallback when key auth fails.
func passwordPrompt() (string, error) {
	fmt.Print("password: ")

	// Read password without echo — simple approach for act_0.
	// In act_2 we'll use golang.org/x/term for proper terminal handling.
	var password string

	if _, err := fmt.Scanln(&password); err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	return password, nil
}

// ─────────────────────────────────────────────
// SFTP operations
// ─────────────────────────────────────────────

// downloadToTemp fetches the remote file to a local temp file and returns its path.
// If the remote file does not exist, an empty temp file is created instead.
func downloadToTemp(c *sftp.Client, remotePath string) (string, error) {
	// Use the remote file's base name as a prefix so the editor shows a useful title.
	prefix := "ned-" + filepath.Base(remotePath) + "-"

	tmp, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	defer tmp.Close()

	remote, err := c.Open(remotePath)
	if err != nil {
		if isNotExist(err) {
			// File doesn't exist on server — start with an empty local file.
			fmt.Printf("→ %s not found on server, creating new file\n", remotePath)
			return tmp.Name(), nil
		}

		return "", fmt.Errorf("open remote %s: %w", remotePath, err)
	}

	defer remote.Close()

	if _, err = io.Copy(tmp, remote); err != nil {
		return "", fmt.Errorf("copy remote → temp: %w", err)
	}

	return tmp.Name(), nil
}

// uploadFromTemp writes the local temp file back to the remote path.
// Intermediate directories are created if they don't exist.
func uploadFromTemp(c *sftp.Client, localPath, remotePath string) error {
	// Ensure remote parent directories exist.
	remoteDir := filepath.Dir(remotePath)

	if err := c.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("mkdir -p %s: %w", remoteDir, err)
	}

	local, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}

	defer local.Close()

	remote, err := c.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}

	defer remote.Close()

	if _, err = io.Copy(remote, local); err != nil {
		return fmt.Errorf("copy temp → remote: %w", err)
	}

	return nil
}

// isNotExist reports whether the SFTP error means the file was not found.
func isNotExist(err error) bool {
	// sftp wraps os.ErrNotExist; check both.
	return errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "does not exist")
}

// ─────────────────────────────────────────────
// Editor
// ─────────────────────────────────────────────

// openEditor opens the given file in the user's preferred editor and waits for it to exit.
// Editor resolution order: $EDITOR env var → nvim → vim → nano.
func openEditor(path string) error {
	editor := resolveEditor()

	fmt.Printf("→ opening %s in %s\n", filepath.Base(path), editor)

	cmd := exec.Command(editor, path)

	// Connect editor to the real terminal so it renders properly.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %q exited with error: %w", editor, err)
	}

	return nil
}

// resolveEditor finds the first available editor from the preference chain.
func resolveEditor() string {
	// Respect the user's explicit preference first.
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}

	// Fall through common editors in order of preference.
	for _, candidate := range []string{"nvim", "vim", "nano", "vi"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}

	// If nothing is found, return "vi" and let the OS error be descriptive.
	return "vi"
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

// currentUser returns the current OS username, falling back to "root".
func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}

	if u := os.Getenv("LOGNAME"); u != "" {
		return u
	}

	return "root"
}

// expandHome replaces a leading "~" with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	return filepath.Join(home, path[1:])
}

// fatalf prints an error message and exits with code 1.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
	os.Exit(1)
}

// printUsage prints a short usage message to stderr.
func printUsage() {
	fmt.Fprintln(os.Stderr, "ned — open a remote file in your local editor over SSH")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  ned [user@]host[:port]:/remote/path")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "examples:")
	fmt.Fprintln(os.Stderr, "  ned root@192.168.1.10:/etc/nginx/nginx.conf")
	fmt.Fprintln(os.Stderr, "  ned deploy@prod.example.com:/app/.env")
	fmt.Fprintln(os.Stderr, "  ned 10.0.0.1:2222:/tmp/test.txt")
}
