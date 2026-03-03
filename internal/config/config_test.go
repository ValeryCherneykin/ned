package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ValeryCherneykin/ned/internal/config"
)

// writeConfig creates a temp config file with the given YAML content and
// temporarily sets HOME so config.Load() picks it up.
func writeConfig(t *testing.T, content string) {
	t.Helper()

	tmpHome := t.TempDir()
	nedDir := filepath.Join(tmpHome, ".ned")

	if err := os.MkdirAll(nedDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(nedDir, "config.yml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Override HOME so config.Load resolves the temp directory.
	t.Setenv("HOME", tmpHome)
}

func TestLoad_MissingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
}

func TestLoad_Defaults(t *testing.T) {
	writeConfig(t, `
defaults:
  user: valery
  port: "2222"
  identity: ~/.ssh/ned_id_ed25519
`)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Defaults.User != "valery" {
		t.Errorf("Defaults.User = %q, want %q", cfg.Defaults.User, "valery")
	}

	if cfg.Defaults.Port != "2222" {
		t.Errorf("Defaults.Port = %q, want %q", cfg.Defaults.Port, "2222")
	}

	if cfg.Defaults.Identity != "~/.ssh/ned_id_ed25519" {
		t.Errorf("Defaults.Identity = %q, want %q", cfg.Defaults.Identity, "~/.ssh/ned_id_ed25519")
	}
}

func TestLoad_HostAlias(t *testing.T) {
	writeConfig(t, `
hosts:
  prod:
    host: 192.168.1.10
    user: deploy
    port: "22"
    identity: ~/.ssh/prod_key
  dev:
    host: 10.0.0.5
    user: root
`)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	prod, ok := cfg.ResolveAlias("prod")
	if !ok {
		t.Fatal("ResolveAlias(prod) returned false")
	}

	if prod.Host != "192.168.1.10" {
		t.Errorf("prod.Host = %q, want %q", prod.Host, "192.168.1.10")
	}

	if prod.User != "deploy" {
		t.Errorf("prod.User = %q, want %q", prod.User, "deploy")
	}
}

func TestResolveAlias_NotFound(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	_, ok := cfg.ResolveAlias("nonexistent")
	if ok {
		t.Error("ResolveAlias(nonexistent) returned true, want false")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	writeConfig(t, `this: is: not: valid: yaml: [[[`)

	_, err := config.Load()
	if err == nil {
		t.Error("Load() expected error for invalid YAML, got nil")
	}
}
