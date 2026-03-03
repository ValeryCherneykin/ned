# act_3 — Tests & Benchmarks

> Goal: prove every package works correctly, find real bottlenecks before optimizing.
> No new features. No guessing. Only data.

## What this act covers

### Unit tests
- `internal/target`   — parse valid/invalid inputs, SSH and Docker schemes
- `internal/config`   — load YAML, alias resolution, missing file handling
- `internal/transfer` — download existing/missing files, upload, error paths
- `internal/auth`     — method chain order, identity file override
- `internal/keygen`   — key pair generation, idempotency, server install script
- `internal/editor`   — editor resolution order ($EDITOR, fallback chain)

### Integration tests
- `internal/backend/docker` — real docker exec against a live container (skipped in CI if docker absent)
- `internal/connection`     — real SSH dial against localhost:2222 (skipped if no server)

### Benchmarks
- `BenchmarkTargetParse`     — how fast is arg parsing
- `BenchmarkDownload`        — SFTP download speed at various file sizes (1KB, 1MB, 10MB)
- `BenchmarkUpload`          — SFTP upload speed at various file sizes
- `BenchmarkDockerReadFile`  — docker exec overhead vs SFTP

### Race detector
All tests run with `-race` — zero data races accepted.

## Test infrastructure

### Backend mock
```go
// internal/backend/mock/mock.go
// InMemoryBackend — implements backend.Backend with a map[string][]byte.
// Used in all transfer tests — no real SSH/Docker needed.
```

### SSH test server
```go
// internal/testutil/sshserver/sshserver.go
// Spins up a real in-process SSH server on a random port.
// Used in connection and auth integration tests.
```

## Coverage target

| Package    | Target |
|------------|--------|
| target     | 100%   |
| config     | 95%+   |
| transfer   | 95%+   |
| auth       | 90%+   |
| keygen     | 90%+   |
| editor     | 85%+   |
| backend    | 90%+   |

## Commands

```bash
task test              # run all tests
task test-race         # run with race detector
task test-coverage     # generate coverage.html
task bench             # run all benchmarks
task bench-save        # save results to bench.txt for act_4 comparison
```

## Commit message

```
test(act_3): unit tests, benchmarks, mock backend, in-process ssh server
```
