# act_0 — Working Monolith

> Goal: one file, zero abstractions, fully working.
> Prove the concept end-to-end before touching architecture.

## What this act covers

- Parse CLI argument `[user@]host:/path/to/file`
- Connect via SSH (private key auth → password fallback)
- Download remote file to a local temp file via SFTP
- If file does not exist on server — create it (empty)
- Detect and open the user's preferred editor (`$EDITOR` → `nvim` → `vim` → `nano`)
- Wait for editor to exit
- Upload the modified temp file back to the exact remote path
- Clean up temp file on exit (even on error)
- Print clear status messages at each step

## What this act does NOT cover

- Config file (`~/.ned/config.yml`) — act_2
- SSH agent / jump hosts — act_2
- Concurrent editing locks — act_3
- File permission preservation — act_3
- Tests / mocks — act_4
- CI / release pipeline — act_5

## Acceptance criteria

```
ned valery@192.168.1.10:/etc/app/.env
```

1. Connects to host
2. Downloads `.env` (or creates empty if missing)
3. Opens in local Neovim / Vim
4. On `:wq` — uploads changes back
5. File on server reflects edits

## Files changed

- `main.go` — entire implementation
- `go.mod` — module definition
- `go.sum` — locked deps

## Dependencies

| Package | Purpose |
|---------|---------|
| `golang.org/x/crypto/ssh` | SSH connection & auth |
| `github.com/pkg/sftp` | SFTP file transfer |

## Commit message

```
feat(act_0): working monolith — ssh connect, sftp rw, local editor loop
```
