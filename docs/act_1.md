# act_1 — Proper Package Structure

> Goal: split the monolith into clean, testable, single-responsibility packages.
> Zero new features — same behaviour, better architecture.

## Structure

```
ned/
├── cmd/
│   └── ned/
│       └── main.go          # entry point — flags, wiring, nothing else
├── internal/
│   ├── target/
│   │   └── target.go        # parse [user@]host[:port]:/path
│   ├── auth/
│   │   └── auth.go          # SSH auth chain: agent → keys → password
│   ├── connection/
│   │   └── connection.go    # SSH dial + SFTP client init
│   ├── transfer/
│   │   └── transfer.go      # SFTP download to temp / upload from temp
│   ├── editor/
│   │   └── editor.go        # resolve $EDITOR, open, wait
│   └── terminal/
│       └── terminal.go      # password prompt, status printing
├── main.go                  # thin shim → cmd/ned
├── go.mod
├── go.sum
├── Taskfile.yml
└── .golangci.yml
```

## Rules for each package

| Package | Responsibility | Knows about |
|---------|---------------|-------------|
| `target` | parse CLI arg | nothing |
| `auth` | build `[]ssh.AuthMethod` | `terminal` |
| `connection` | dial SSH, init SFTP | `target`, `auth` |
| `transfer` | download / upload file | nothing (takes sftp.Client) |
| `editor` | open local editor | nothing |
| `terminal` | I/O: prompts, status | nothing |
| `cmd/ned` | wire everything | all internal |

## What changes vs act_0

- `main.go` shrinks to ~10 lines
- Each package is independently testable
- No package imports its sibling except `cmd/ned`
- All errors bubble up — no `os.Exit` outside `cmd/ned`
- `fatalf` moves to `cmd/ned`, internal packages return errors only

## What does NOT change

- Zero new features
- Same CLI syntax
- Same auth flow
- Same editor behaviour

## Acceptance criteria

```
ned root@localhost:2222:/tmp/test.txt
```
Identical behaviour to act_0 — just cleaner internals.

## Commit message

```
refactor(act_1): split monolith into internal packages
```
