// Command ned opens a remote file in your local editor over SSH/SFTP or Docker.
//
// Usage:
//
//	ned [flags] [user@]host[:port]:/remote/path
//	ned [flags] docker://container:/remote/path
//
// Examples:
//
//	ned root@192.168.1.10:/etc/nginx/nginx.conf
//	ned -i ~/.ssh/prod deploy@prod.example.com:/app/.env
//	ned docker://my-container:/app/config.json
//	ned prod:/etc/.env
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ValeryCherneykin/ned/internal/backend"
	"github.com/ValeryCherneykin/ned/internal/config"
	"github.com/ValeryCherneykin/ned/internal/connection"
	"github.com/ValeryCherneykin/ned/internal/editor"
	"github.com/ValeryCherneykin/ned/internal/keygen"
	"github.com/ValeryCherneykin/ned/internal/recovery"
	"github.com/ValeryCherneykin/ned/internal/target"
	"github.com/ValeryCherneykin/ned/internal/terminal"
	"github.com/ValeryCherneykin/ned/internal/transfer"
)

// Version is set at build time via -ldflags "-X main.Version=v1.0.0".
var Version = "dev"

func main() {
	identityFile := flag.String("i", "", "SSH identity file")
	portOverride := flag.String("p", "", "SSH port override")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = terminal.PrintUsage
	flag.Parse()

	if *showVersion {
		fmt.Printf("ned %s\n", Version)
		os.Exit(0)
	}

	if flag.NArg() != 1 {
		terminal.PrintUsage()
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	run(ctx, flag.Arg(0), *identityFile, *portOverride)
}

// Options holds resolved connection parameters built from flags + config.
type Options struct {
	IdentityFile string
	Port         string
}

func run(ctx context.Context, raw, identityFile, portOverride string) {
	// ── Load config ──────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		terminal.Warn("config load error: %v (continuing without config)", err)
		cfg = &config.Config{}
	}

	// ── Resolve alias + parse target ─────────────────────────────────────────
	resolved := resolveAlias(raw, cfg, portOverride)

	t, err := target.Parse(resolved)
	if err != nil {
		terminal.Fatal("invalid target %q: %v", raw, err)
	}

	if portOverride != "" {
		t.Port = portOverride
	}

	opts := buildOptions(cfg, identityFile)

	// ── Open backend ─────────────────────────────────────────────────────────
	var b backend.Backend

	if t.IsDocker() {
		terminal.Status("connecting to docker container %s", t.Host)
		b = backend.NewDocker(t.Host)
	} else {
		terminal.Status("connecting %s@%s:%s", t.User, t.Host, t.Port)

		var connOpts []connection.Option
		if opts.IdentityFile != "" {
			connOpts = append(connOpts, connection.WithIdentityFile(opts.IdentityFile))
		}

		sess, dialErr := connection.Open(t, connOpts...)
		if dialErr != nil {
			terminal.Fatal("connection failed: %v", dialErr)
		}

		defer func() {
			if closeErr := sess.Close(); closeErr != nil {
				terminal.Warn("close session: %v", closeErr)
			}
		}()

		terminal.Status("connected")
		offerKeyInstall(sess, t)
		b = backend.NewSSH(sess.SFTP)
	}

	// ── Download ─────────────────────────────────────────────────────────────
	tmpPath, isNew, err := transfer.Download(b, t.RemotePath)
	if err != nil {
		terminal.Fatal("download: %v", err)
	}

	removeTmp := true

	defer func() {
		if removeTmp {
			if removeErr := os.Remove(tmpPath); removeErr != nil {
				terminal.Warn("remove temp: %v", removeErr)
			}
		}
	}()

	if isNew {
		terminal.Status("%s not found, creating new file", t.RemotePath)
	} else {
		terminal.Status("%s downloaded", t.RemotePath)
	}

	// ── Edit ─────────────────────────────────────────────────────────────────
	terminal.Status("opening in %s", editor.Resolved())

	if err = editor.Open(tmpPath); err != nil {
		select {
		case <-ctx.Done():
			terminal.Warn("interrupted — discarding changes")
			return
		default:
		}

		terminal.Fatal("editor: %v", err)
	}

	// ── Upload ───────────────────────────────────────────────────────────────
	terminal.Status("uploading to %s", t.RemotePath)

	uploadErr := transfer.Upload(b, tmpPath, t.RemotePath)

	// Check if a signal arrived during upload.
	select {
	case <-ctx.Done():
		if uploadErr == nil {
			terminal.Success("saved %s", t)
		} else {
			handleUploadFailure(tmpPath, t.RemotePath, &removeTmp)
		}

		return
	default:
	}

	if uploadErr != nil {
		handleUploadFailure(tmpPath, t.RemotePath, &removeTmp)
		os.Exit(1)
	}

	terminal.Success("saved %s", t)
}

// handleUploadFailure saves changes to ~/.ned/recovery/ so nothing is lost.
func handleUploadFailure(tmpPath, remotePath string, removeTmp *bool) {
	*removeTmp = false

	savedPath, saveErr := recovery.Save(tmpPath, remotePath)
	if saveErr != nil {
		terminal.Warn("upload failed AND recovery save failed: %v", saveErr)
		terminal.Warn("your edits are in temp file: %s", tmpPath)
		return
	}

	*removeTmp = true

	terminal.Warn("upload failed — your changes are saved at:")
	terminal.Warn("  %s", savedPath)
}

// offerKeyInstall offers to generate and install an SSH key for passwordless access.
func offerKeyInstall(sess *connection.Session, t target.Target) {
	kp, err := keygen.DefaultKeyPair()
	if err != nil {
		return
	}

	if _, err = os.Stat(kp.PrivatePath); err == nil {
		return
	}

	if !terminal.Confirm(
		fmt.Sprintf("No SSH key found for %s. Install one for passwordless access?", t.Host),
		true,
	) {
		return
	}

	kp, err = keygen.EnsureKeyPair(os.Stdout)
	if err != nil {
		terminal.Warn("key generation failed: %v", err)
		return
	}

	if err = keygen.InstallOnServer(kp.PublicPath, sess.RunCommand); err != nil {
		terminal.Warn("key install failed: %v (add it manually)", err)
		return
	}

	terminal.Success("SSH key installed — next connect will be passwordless")
}

// resolveAlias expands a config alias to a full target string.
func resolveAlias(raw string, cfg *config.Config, portOverride string) string {
	if strings.Contains(raw, ":/") {
		return raw
	}

	h, ok := cfg.ResolveAlias(raw)
	if !ok {
		return raw
	}

	user := h.User
	if user == "" {
		user = cfg.Defaults.User
	}

	port := h.Port
	if port == "" {
		port = cfg.Defaults.Port
	}

	if portOverride != "" {
		port = portOverride
	}

	switch {
	case user != "" && port != "":
		return fmt.Sprintf("%s@%s:%s:/", user, h.Host, port)
	case user != "":
		return fmt.Sprintf("%s@%s:/", user, h.Host)
	default:
		return fmt.Sprintf("%s:/", h.Host)
	}
}

// buildOptions merges CLI flags with config defaults.
func buildOptions(cfg *config.Config, identityFile string) Options {
	opts := Options{IdentityFile: identityFile}

	if opts.IdentityFile == "" && cfg.Defaults.Identity != "" {
		opts.IdentityFile = cfg.Defaults.Identity
	}

	return opts
}
