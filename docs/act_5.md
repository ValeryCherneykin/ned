# act_5 — Watch Mode (`-w`)

## Goal

`ned -w docker://ned-test:/etc/nginx.conf`

Edit → `:w` → live upload in ~100ms → stay in vim → repeat → `:wq` → done.

No workflow change. Базовый режим (без `-w`) работает как раньше.

---

## User Experience

```
$ ned -w docker://ned-test:/etc/nginx.conf
→ connecting to docker container ned-test
→ /etc/nginx.conf downloaded
→ opening in nvim (watch mode)
↑ saved /etc/nginx.conf  (every :w)
↑ saved /etc/nginx.conf
✓ saved docker://ned-test:/etc/nginx.conf
```

---

## Implementation

### New dependency

```
go get github.com/fsnotify/fsnotify
```

Works on macOS (FSEvents / kqueue) and Linux (inotify) — no platform code needed.

### New package: `internal/watch/watch.go`

```
Watch(ctx, tmpPath, remotePath, backend) error
```

- запускает `fsnotify.Watcher` на `tmpPath`
- на событие `Write` или `Create` — вызывает `transfer.Upload`
- печатает `↑ saved <remotePath>` после каждого upload
- останавливается когда `ctx` отменён (editor вышел)

### Changes to `main.go`

1. Добавить флаг `-w bool`
2. В `run()` — если `-w`:
   - создать `context.WithCancel`
   - запустить `watch.Watch(ctx, ...)` в горутине
   - открыть editor
   - после выхода editor — отменить ctx, дождаться горутины
   - финальный upload (как сейчас)

---

## Files Changed

| File | Change |
|------|--------|
| `internal/watch/watch.go` | новый пакет |
| `internal/watch/watch_test.go` | тесты |
| `cmd/ned/main.go` | флаг `-w`, вызов watch |
| `go.mod` / `go.sum` | fsnotify |

---

## Commit

```
feat(act_5): watch mode — live upload on :w via fsnotify (#-w)
```
