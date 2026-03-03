# act_2 — Config, Flags, Docker, Auto SSH Key

> Goal: make ned production-ready for daily use.
> No more typing passwords, no more remembering IPs, Docker just works.

## New features

### 1. Config file `~/.ned/config.yml`
```yaml
defaults:
  user: valery
  port: 22
  identity: ~/.ssh/ned_id_ed25519

hosts:
  prod:
    host: 192.168.1.10
    user: deploy
    port: 22
    identity: ~/.ssh/prod_key
  dev:
    host: 10.0.0.5
    user: root
```

```bash
ned prod:/etc/.env                   # alias from config
ned root@192.168.1.10:/etc/.env      # still works
```

### 2. Flags
```bash
ned -i ~/.ssh/mykey root@host:/path
ned -p 2222 root@host:/path
```

### 3. Docker backend
```bash
ned docker://container-name:/etc/nginx/nginx.conf
```
Uses `docker exec` — no SSH, no ports, no keys.

### 4. Auto SSH key setup
1. ned tries key auth → fails
2. Asks password (hidden, no echo)
3. After connect → "Install SSH key for passwordless access? [Y/n]"
4. Generates `~/.ssh/ned_id_ed25519` → installs pubkey on server
5. Next connect → zero friction

### 5. Hidden password input
`golang.org/x/term` — characters not echoed to terminal.

## Architecture

### New `Backend` interface
```
internal/backend/
  backend.go    — interface: ReadFile, WriteFile, MkdirAll
  ssh.go        — SFTP implementation
  docker.go     — docker exec implementation
```

### Updated packages
| Package | Change |
|---------|--------|
| `target` | Parse `docker://` scheme |
| `terminal` | Hidden password via x/term |
| `auth` | Accept `-i` flag |
| `config` | New — YAML loader |
| `keygen` | New — ed25519 gen + install |
| `backend` | New — interface + impls |
| `transfer` | Takes Backend interface |
| `cmd/ned` | Flags, config wiring |

## New deps
```
golang.org/x/term
gopkg.in/yaml.v3
```

## Commit message
```
feat(act_2): config file, docker backend, auto ssh key, hidden password, flags
```
