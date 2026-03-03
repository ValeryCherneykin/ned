// Command ned opens a remote file in your local editor over SSH/SFTP.
//
// Usage:
//
//	ned [user@]host[:port]:/remote/path
//
// Examples:
//
//	ned root@192.168.1.10:/etc/nginx/nginx.conf
//	ned deploy@prod.example.com:/app/.env
//	ned 10.0.0.1:2222:/tmp/test.txt
package main

import (
	"os"

	"github.com/ValeryCherneykin/ned/internal/connection"
	"github.com/ValeryCherneykin/ned/internal/editor"
	"github.com/ValeryCherneykin/ned/internal/target"
	"github.com/ValeryCherneykin/ned/internal/terminal"
	"github.com/ValeryCherneykin/ned/internal/transfer"
)

func main() {
	if len(os.Args) != 2 {
		terminal.PrintUsage()
		os.Exit(1)
	}

	run(os.Args[1])
}

// run is the full edit-over-SSH workflow:
//  1. Parse target
//  2. Open SSH + SFTP session
//  3. Download remote file to temp
//  4. Open temp in local editor
//  5. Upload modified temp back to server
//  6. Clean up
func run(raw string) {
	// ── 1. Parse ────────────────────────────────────────────────────────────
	t, err := target.Parse(raw)
	if err != nil {
		terminal.Fatal("invalid target %q: %v", raw, err)
	}

	// ── 2. Connect ──────────────────────────────────────────────────────────
	terminal.Status("connecting %s@%s:%s", t.User, t.Host, t.Port)

	sess, err := connection.Open(t)
	if err != nil {
		terminal.Fatal("connection failed: %v", err)
	}

	defer func() {
		if closeErr := sess.Close(); closeErr != nil {
			terminal.Warn("close session: %v", closeErr)
		}
	}()

	terminal.Status("connected")

	// ── 3. Download ─────────────────────────────────────────────────────────
	tempPath, isNew, err := transfer.Download(sess.SFTP, t.RemotePath)
	if err != nil {
		terminal.Fatal("download failed: %v", err)
	}

	defer func() {
		if removeErr := os.Remove(tempPath); removeErr != nil {
			terminal.Warn("remove temp file: %v", removeErr)
		}
	}()

	if isNew {
		terminal.Status("%s not found on server, creating new file", t.RemotePath)
	} else {
		terminal.Status("%s downloaded", t.RemotePath)
	}

	// ── 4. Edit ─────────────────────────────────────────────────────────────
	terminal.Status("opening in %s", editor.Resolved())

	if err = editor.Open(tempPath); err != nil {
		terminal.Fatal("editor error: %v", err)
	}

	// ── 5. Upload ────────────────────────────────────────────────────────────
	terminal.Status("uploading changes to %s", t.RemotePath)

	if err = transfer.Upload(sess.SFTP, tempPath, t.RemotePath); err != nil {
		terminal.Fatal("upload failed: %v", err)
	}

	terminal.Success("saved %s", t)
}
