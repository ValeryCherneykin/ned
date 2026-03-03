<div align="center">
  <img src="assets/logo.png" alt="ned logo" width="180" />

  <h1>ned</h1>

  <p><strong>Open any remote file in your local editor. One command.</strong></p>

  <p>
    <a href="https://github.com/ValeryCherneykin/ned/actions/workflows/ci.yml">
      <img src="https://github.com/ValeryCherneykin/ned/actions/workflows/ci.yml/badge.svg" alt="CI" />
    </a>
    <a href="https://github.com/ValeryCherneykin/ned/releases/latest">
      <img src="https://img.shields.io/github/v/release/ValeryCherneykin/ned?color=green" alt="Release" />
    </a>
    <a href="https://goreportcard.com/report/github.com/ValeryCherneykin/ned">
      <img src="https://goreportcard.com/badge/github.com/ValeryCherneykin/ned" alt="Go Report" />
    </a>
    <a href="https://opensource.org/licenses/MIT">
      <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License" />
    </a>
  </p>
</div>

---

Stop SSHing into servers just to edit a config file.  
`ned` pulls the file to your machine, opens it in your local editor — Neovim, Vim, whatever you use — then pushes it back. Done.

```bash
ned root@192.168.1.10:/etc/nginx/nginx.conf
```

Works over SSH. Works with Docker containers. No setup required.

---

## Install

**Homebrew** (macOS & Linux)
```bash
brew tap ValeryCherneykin/ned
brew install ned
```

**Go**
```bash
go install github.com/ValeryCherneykin/ned/cmd/ned@latest
```

**Binary** — download from [Releases](https://github.com/ValeryCherneykin/ned/releases/latest) for your platform.

---

## Usage

```
ned [flags] [user@]host[:port]:/remote/path
ned [flags] docker://container:/remote/path
```

**SSH**
```bash
# Basic
ned root@192.168.1.10:/etc/nginx/nginx.conf

# Custom port
ned root@192.168.1.10:2222:/etc/.env

# Explicit SSH key
ned -i ~/.ssh/prod_key deploy@prod.example.com:/app/.env
```

**Docker**
```bash
# Edit a file inside a running container — no SSH needed
ned docker://my-container:/app/config.json
ned docker://postgres:/etc/postgresql/postgresql.conf
```

**Config aliases**
```bash
# Instead of typing the full address every time
ned prod:/etc/.env
ned dev:/app/config.yml
```

**Flags**

| Flag | Description |
|------|-------------|
| `-i <path>` | Path to SSH private key |
| `-p <port>` | SSH port (overrides config and default 22) |
| `--version` | Print version and exit |

---

## First connect

The first time you connect to a host with a password, ned offers to install an SSH key automatically:

```
→ connecting root@192.168.1.10:22
root@192.168.1.10's password: ••••••••
→ connected
No SSH key found for 192.168.1.10. Install one for passwordless access? [Y/n]: y
✓ generated new SSH key: ~/.ssh/ned_id_ed25519
✓ SSH key installed — next connect will be passwordless
```

From that point on, `ned root@192.168.1.10:/any/file` connects instantly with no password.

---

## Config file

Create `~/.ned/config.yml` to define aliases and defaults:

```yaml
defaults:
  user: valery
  port: 22
  identity: ~/.ssh/ned_id_ed25519

hosts:
  prod:
    host: 192.168.1.10
    user: deploy
    identity: ~/.ssh/prod_key
  dev:
    host: 10.0.0.5
    user: root
  staging:
    host: staging.example.com
    port: 2222
    user: ubuntu
```

Then just:
```bash
ned prod:/etc/nginx/nginx.conf
ned dev:/app/.env
ned staging:/var/log/app.log
```

---

## Recovery

If an upload fails mid-way (network drop, server restart), your changes are never lost:

```
✗ upload failed — your changes are saved at:
  ~/.ned/recovery/.env_20260304_153042
```

Fix the connection and re-run ned — your edits are waiting.

---

## How it works

```
ned root@host:/etc/.env
      │
      ├── 1. Parse target
      ├── 2. Connect via SSH (agent → key files → password)
      ├── 3. SFTP download → /tmp/ned-.env-XXXXX
      ├── 4. Open in $EDITOR (nvim / vim / nano)
      ├── 5. Wait for editor to exit
      ├── 6. SFTP upload → /etc/.env
      └── 7. Clean up temp file
```

Signals are handled gracefully — if the process is interrupted during upload, changes are saved to `~/.ned/recovery/` before exit.

---

## Editor

ned respects your `$EDITOR` environment variable. If not set, it tries `nvim → vim → nano → vi`.

```bash
# Use a specific editor for one session
EDITOR=hx ned root@host:/etc/.env
```

---

## Architecture

```
ned/
├── cmd/ned/          # entry point — flags, wiring, graceful shutdown
└── internal/
    ├── auth/         # SSH auth chain: agent → keys → password
    ├── backend/      # Backend interface + SSH/SFTP + Docker implementations
    │   └── mock/     # In-memory backend for tests
    ├── config/       # ~/.ned/config.yml loader and alias resolver
    ├── connection/   # SSH dial + SFTP session lifecycle
    ├── editor/       # $EDITOR resolution and exec
    ├── keygen/       # ed25519 key generation + server install
    ├── recovery/     # Save edits locally on upload failure
    ├── target/       # CLI argument parser (SSH and Docker schemes)
    ├── terminal/     # User-facing I/O: status, prompts, hidden password
    └── transfer/     # Download to temp / upload from temp
```

---

## Contributing

```bash
git clone https://github.com/ValeryCherneykin/ned
cd ned

# Install dev tools
task install-tools

# Run tests
task test

# Run tests with race detector
task test-race

# Lint
task lint

# Full check before PR
task check
```

---

## License

MIT © [Valery Cherneykin](https://github.com/ValeryCherneykin)
