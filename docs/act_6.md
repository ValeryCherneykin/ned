# act_6 — Directory Mode

## Usage

```bash
ned prod:/etc/nginx/        # directory mode (trailing slash or no extension)
ned prod:/etc/nginx/ --sync # sync mode: delete locally → delete remote
```

## Flow

```
1. detect target is directory
2. ReadDir recursively → download all files → /tmp/ned-nginx-XXXXX/
3. open nvim /tmp/ned-nginx-XXXXX/  (netrw opens automatically)
4. watch entire tmpdir via mtime polling
   → changed file  → upload
   → new file      → upload
5. on exit:
   → collect deleted files (were in snapshot, not in tmpdir now)
   → if any deleted: show list, ask confirmation per file
   → --sync: delete remote without asking
6. cleanup tmpdir
```

## Interface Changes

### `internal/backend/backend.go`

Add to Backend interface:

```go
// ReadDir lists the contents of a remote directory (non-recursive).
// Returns os.ErrNotExist if path does not exist.
ReadDir(path string) ([]Entry, error)
```

New type:

```go
// Entry is a single item returned by ReadDir.
type Entry struct {
    Name  string
    IsDir bool
    Size  int64
}
```

### Implementations

| Backend | Implementation |
|---------|---------------|
| SSH     | `sftp.Client.ReadDir()` |
| Docker  | `docker exec ls -la --full-time` parsed |
| Mock    | in-memory map scan |

## New Package: `internal/dirmode/`

```
internal/dirmode/
├── dirmode.go       # Download, Watch, Snapshot, CollectDeleted
└── dirmode_test.go
```

### `Download(b Backend, remotePath, localDir string) error`
Recursively walks remote dir via ReadDir, downloads each file.

### `Snapshot(localDir string) (map[string]time.Time, error)`
Records mtime of every file in tmpdir — used to detect new/changed/deleted.

### `Watch(ctx, localDir, remoteBase, b Backend) error`
Polls localDir every 500ms:
- mtime changed → upload
- new file → upload
- file gone → add to deletedList

### `CollectDeleted(snapshot, current map[string]time.Time) []string`
Returns paths that existed in snapshot but not in current.

## Changes to `cmd/ned/main.go`

```go
if strings.HasSuffix(t.RemotePath, "/") || isDir(b, t.RemotePath) {
    runDirMode(b, t, syncMode)
} else {
    runFileMode(b, t, watchMode)
}
```

New flags:
- `--sync` bool

## Files Changed

| File | Change |
|------|--------|
| `internal/backend/backend.go` | add Entry, ReadDir to interface |
| `internal/backend/ssh.go` | implement ReadDir |
| `internal/backend/docker.go` | implement ReadDir |
| `internal/backend/mock/mock.go` | implement ReadDir |
| `internal/dirmode/dirmode.go` | new package |
| `internal/dirmode/dirmode_test.go` | tests |
| `cmd/ned/main.go` | dir detection, --sync flag, runDirMode |

## Commit

```
feat(act_6): directory mode — download, watch, upload, deleted confirmation
```
