package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ValeryCherneykin/ned/internal/backend"
	"github.com/ValeryCherneykin/ned/internal/config"
	"github.com/ValeryCherneykin/ned/internal/connection"
	"github.com/ValeryCherneykin/ned/internal/editor"
	"github.com/ValeryCherneykin/ned/internal/keygen"
	"github.com/ValeryCherneykin/ned/internal/target"
	"github.com/ValeryCherneykin/ned/internal/terminal"
	"github.com/ValeryCherneykin/ned/internal/transfer"
	"github.com/ValeryCherneykin/ned/internal/watch"
)

// Version is set at build time via -ldflags "-X main.Version=x.y.z".
var Version = "dev"

func main() {
	identityFile := flag.String("i", "", "SSH identity file")
	portOverride := flag.String("p", "", "SSH port override")
	watchMode := flag.Bool("w", false, "watch mode: upload on every :w without exiting the editor")
	flag.Usage = terminal.PrintUsage
	flag.Parse()

	if flag.NArg() == 1 && flag.Arg(0) == "--version" {
		fmt.Println(Version)
		return
	}

	if flag.NArg() != 1 {
		terminal.PrintUsage()
		os.Exit(1)
	}

	run(flag.Arg(0), *identityFile, *portOverride, *watchMode)
}

func run(raw, identityFile, portOverride string, watchMode bool) {
	// ── Load config ──────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		terminal.Warn("config load error: %v (continuing without config)", err)
		cfg = &config.Config{}
	}

	// ── Parse target ─────────────────────────────────────────────────────────
	resolved := resolveAlias(raw, cfg, portOverride)

	t, err := target.Parse(resolved)
	if err != nil {
		terminal.Fatal("invalid target %q: %v", raw, err)
	}

	if portOverride != "" {
		t.Port = portOverride
	}

	applyDefaults(&t, cfg, &identityFile)

	// ── Open backend ─────────────────────────────────────────────────────────
	var b backend.Backend

	if t.IsDocker() {
		terminal.Status("connecting to docker container %s", t.Host)
		b = backend.NewDocker(t.Host)
	} else {
		terminal.Status("connecting %s@%s:%s", t.User, t.Host, t.Port)

		var connOpts []connection.Option
		if identityFile != "" {
			connOpts = append(connOpts, connection.WithIdentityFile(identityFile))
		}

		sess, openErr := connection.Open(t, connOpts...)
		if openErr != nil {
			terminal.Fatal("connection failed: %v", openErr)
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

	defer func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			terminal.Warn("remove temp: %v", removeErr)
		}
	}()

	if isNew {
		terminal.Status("%s not found, creating new file", t.RemotePath)
	} else {
		terminal.Status("%s downloaded", t.RemotePath)
	}

	// ── Edit ─────────────────────────────────────────────────────────────────
	if watchMode {
		terminal.Status("opening in %s (watch mode — uploading on every :w)", editor.Resolved())
		runWithWatch(b, tmpPath, t)
	} else {
		terminal.Status("opening in %s", editor.Resolved())

		if err = editor.Open(tmpPath); err != nil {
			terminal.Fatal("editor: %v", err)
		}

		terminal.Status("uploading to %s", t.RemotePath)

		if err = transfer.Upload(b, tmpPath, t.RemotePath); err != nil {
			terminal.Fatal("upload: %v", err)
		}
	}

	terminal.Success("saved %s", t)
}

// runWithWatch starts the fsnotify watcher in a goroutine, opens the editor,
// and cancels the watcher when the editor exits. A final upload is always done
// on exit to ensure the last :wq write is captured.
func runWithWatch(b backend.Backend, tmpPath string, t target.Target) {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		if err := watch.Watch(ctx, tmpPath, t.RemotePath, b); err != nil {
			terminal.Warn("watcher: %v", err)
		}
	}()

	if err := editor.Open(tmpPath); err != nil {
		terminal.Fatal("editor: %v", err)
	}

	cancel()
	wg.Wait()

	// Final upload — ensures the last :wq write is always uploaded.
	terminal.Status("uploading to %s", t.RemotePath)

	if err := transfer.Upload(b, tmpPath, t.RemotePath); err != nil {
		terminal.Fatal("upload: %v", err)
	}
}

// offerKeyInstall offers to install an SSH key after password-auth success.
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
		terminal.Warn("key install failed: %v (you can add it manually)", err)
		return
	}

	terminal.Success("SSH key installed — next connect will be passwordless")
}

// resolveAlias expands a config alias to a full target string.
func resolveAlias(raw string, cfg *config.Config, portOverride string) string {
	if strings.Contains(raw, ":/") {
		return raw
	}

	host, ok := cfg.ResolveAlias(raw)
	if !ok {
		return raw
	}

	user := host.User
	if user == "" {
		user = cfg.Defaults.User
	}

	port := host.Port
	if port == "" {
		port = cfg.Defaults.Port
	}

	if portOverride != "" {
		port = portOverride
	}

	if user != "" && port != "" {
		return fmt.Sprintf("%s@%s:%s:/", user, host.Host, port)
	}

	if user != "" {
		return fmt.Sprintf("%s@%s:/", user, host.Host)
	}

	return fmt.Sprintf("%s:/", host.Host)
}

// applyDefaults fills in zero fields from the config defaults.
func applyDefaults(t *target.Target, cfg *config.Config, identityFile *string) {
	if t.User == "" && cfg.Defaults.User != "" {
		t.User = cfg.Defaults.User
	}

	if *identityFile == "" && cfg.Defaults.Identity != "" {
		*identityFile = cfg.Defaults.Identity
	}
}
