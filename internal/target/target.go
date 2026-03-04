// Package target parses the CLI argument into its components.
//
// Supported formats:
//
//	SSH:    [user@]host[:port]:/remote/path
//	Docker: docker://container-name:/remote/path
package target

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

const (
	// DefaultPort is the SSH port used when no port is specified.
	DefaultPort = "22"

	// SchemeSSH identifies an SSH/SFTP target.
	SchemeSSH = "ssh"

	// SchemeDocker identifies a Docker exec target.
	SchemeDocker = "docker"
)

// Target holds the parsed components of a ned CLI argument.
type Target struct {
	// Scheme is either SchemeSSH or SchemeDocker.
	Scheme string
	// User is the SSH login name (empty for Docker targets).
	User string
	// Host is the SSH hostname or Docker container name.
	Host string
	// Port is the SSH port (empty for Docker targets).
	Port string
	// RemotePath is the absolute path of the file on the remote.
	RemotePath string
}

// IsDocker reports whether this target refers to a Docker container.
func (t Target) IsDocker() bool { return t.Scheme == SchemeDocker }

// Addr returns "host:port" suitable for net.Dial (SSH targets only).
func (t Target) Addr() string { return net.JoinHostPort(t.Host, t.Port) }

// String returns a human-readable representation of the target.
func (t Target) String() string {
	if t.IsDocker() {
		return fmt.Sprintf("docker://%s%s", t.Host, t.RemotePath)
	}

	return fmt.Sprintf("%s@%s:%s%s", t.User, t.Host, t.Port, t.RemotePath)
}

// Parse parses raw into a Target.
// Accepts SSH ([user@]host[:port]:/path) and Docker (docker://container:/path) formats.
func Parse(raw string) (Target, error) {
	if raw == "" {
		return Target{}, errors.New("target cannot be empty")
	}

	if strings.HasPrefix(raw, "docker://") {
		return parseDocker(raw)
	}

	return parseSSH(raw)
}

func parseDocker(raw string) (Target, error) {
	without := strings.TrimPrefix(raw, "docker://")

	idx := strings.Index(without, ":/")
	if idx == -1 {
		return Target{}, errors.New("docker target: missing ':/' — expected docker://container:/path")
	}

	container := without[:idx]
	path := without[idx+1:]

	if container == "" {
		return Target{}, errors.New("container name cannot be empty")
	}

	return Target{Scheme: SchemeDocker, Host: container, RemotePath: path}, nil
}

func parseSSH(raw string) (Target, error) {
	idx := strings.Index(raw, ":/")
	if idx == -1 {
		return Target{}, errors.New("missing ':/' separator — expected [user@]host:/path")
	}

	hostSection := raw[:idx]
	remotePath := raw[idx+1:]

	user, hostPort, err := splitUserHost(hostSection)
	if err != nil {
		return Target{}, err
	}

	host, port, err := splitHostPort(hostPort)
	if err != nil {
		return Target{}, err
	}

	if user == "" {
		user = currentUser()
	}

	return Target{Scheme: SchemeSSH, User: user, Host: host, Port: port, RemotePath: remotePath}, nil
}

func splitUserHost(s string) (user, hostPort string, err error) {
	at := strings.LastIndex(s, "@")
	if at == -1 {
		return "", s, nil
	}

	user = s[:at]
	if user == "" {
		return "", "", errors.New("user cannot be empty when '@' is present")
	}

	return user, s[at+1:], nil
}

func splitHostPort(hostPort string) (host, port string, err error) {
	if hostPort == "" {
		return "", "", errors.New("host cannot be empty")
	}

	h, p, e := net.SplitHostPort(hostPort)
	if e != nil {
		// No port specified — treat the whole string as the host.
		return hostPort, DefaultPort, nil
	}

	if h == "" {
		return "", "", errors.New("host cannot be empty")
	}

	if p == "" {
		p = DefaultPort
	}

	return h, p, nil
}

// currentUser returns the OS username from environment variables.
// Falls back to "root" when neither $USER nor $LOGNAME is set.
func currentUser() string {
	for _, env := range []string{"USER", "LOGNAME"} {
		if u := os.Getenv(env); u != "" {
			return u
		}
	}

	return "root"
}
