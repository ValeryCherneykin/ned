# act_4 — Optimization

> Rule: no guessing. Every change is justified by act_3 benchmark data.
> After each change: re-bench, compare with benchstat, keep only wins.

## Baseline (act_3, Apple M1 Pro)

```
BenchmarkDownload_1KB    117µs   11 allocs   8.7  MB/s
BenchmarkDownload_1MB    457µs   11 allocs   2293 MB/s
BenchmarkDownload_10MB   4.4ms   11 allocs   2366 MB/s
BenchmarkUpload_1KB      13µs    6  allocs   78   MB/s
BenchmarkUpload_1MB      527µs   30 allocs   1987 MB/s
BenchmarkUpload_10MB     3.2ms   40 allocs   3218 MB/s
BenchmarkParse           50ns    0  allocs   ← already perfect
BenchmarkParseDocker     17ns    0  allocs   ← already perfect
```

## Problems identified

### 1. Upload allocs grow with file size (6 → 30 → 40)
**Cause:** `io.ReadAll` in `DockerBackend.WriteFile` reads entire file into memory,
then creates a `bytes.NewReader` — two full copies of the data in memory.

**Fix:** stream directly via `io.Pipe` + goroutine. Zero intermediate buffer.

### 2. Download 1KB is slow relative to larger files (8.7 MB/s vs 2300 MB/s)
**Cause:** `os.CreateTemp` + syscall overhead dominates for small files.
The 11 allocs are constant regardless of file size — all from temp file creation.

**Fix:** `sync.Pool` of reusable `[]byte` buffers passed to `io.CopyBuffer`
to eliminate the default 32KB heap allocation inside `io.Copy`.

### 3. io.Copy uses default 32KB stack buffer — allocates every call
**Fix:** `sync.Pool` for copy buffers across all transfer operations.

### 4. Keygen prints to stdout during tests (noise)
**Fix:** inject `io.Writer` into keygen functions; tests pass `io.Discard`.

## Changes

| Package | Change | Expected gain |
|---------|--------|--------------|
| `transfer` | `io.CopyBuffer` + `sync.Pool` for copy buf | -1 alloc per op |
| `backend/docker` | streaming write via `io.Pipe` instead of `io.ReadAll` | -20+ allocs on large files |
| `keygen` | injectable writer for status output | cleaner test output |

## What we do NOT touch

- `target.Parse` — already 0 allocs, nothing to do
- `connection` — SSH handshake is network-bound, not CPU-bound
- `editor` — single exec.Command, not in hot path
- `config` — loaded once at startup, irrelevant

## Verification

```bash
# Save new results
go test -bench=. -benchmem -count=10 ./... | tee bench_act4.txt

# Compare with act_3 baseline
benchstat bench_act3.txt bench_act4.txt
```

## Commit message

```
perf(act_4): pool copy buffers, stream docker writes, zero alloc hot paths
```
