package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Tee.Mode != "failures" {
		t.Errorf("expected tee mode 'failures', got %q", cfg.Tee.Mode)
	}
	if cfg.Tee.MaxFiles != 20 {
		t.Errorf("expected max_files 20, got %d", cfg.Tee.MaxFiles)
	}
	if !cfg.Display.Color {
		t.Error("expected color enabled by default")
	}
	if cfg.Tracking.DBPath == "" {
		t.Error("expected non-empty db path")
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("SNIP_CONFIG", "/tmp/nonexistent-snip-config-test.toml")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tee.Mode != "failures" {
		t.Errorf("expected defaults when file missing, got tee.mode=%q", cfg.Tee.Mode)
	}
}

func TestDefaultConfigQuietAndEnable(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Display.QuietNoFilter != false {
		t.Error("expected QuietNoFilter false by default")
	}
	if cfg.Filters.Enable != nil {
		t.Errorf("expected Filters.Enable nil by default, got %v", cfg.Filters.Enable)
	}
}

func TestLoadConfigWithEnable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[display]
quiet_no_filter = true

[filters.enable]
git-diff = false
git-status = true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Display.QuietNoFilter {
		t.Error("expected QuietNoFilter true")
	}
	if cfg.Filters.Enable == nil {
		t.Fatal("expected non-nil Filters.Enable")
	}
	if cfg.Filters.Enable["git-diff"] != false {
		t.Error("expected git-diff disabled")
	}
	if cfg.Filters.Enable["git-status"] != true {
		t.Error("expected git-status enabled")
	}
}

func TestLoadConfigEmptyEnable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[filters.enable]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// nil or empty map both mean "all enabled"
	if len(cfg.Filters.Enable) != 0 {
		t.Errorf("expected nil or empty Filters.Enable, got %v", cfg.Filters.Enable)
	}
}

func TestExpandTildeInPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[tracking]
db_path = "~/.local/share/snip/tracking.db"

[filters]
dir = "~/.config/snip/filters"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedDB := filepath.Join(home, ".local/share/snip/tracking.db")
	if cfg.Tracking.DBPath != expectedDB {
		t.Errorf("db_path: got %q, want %q", cfg.Tracking.DBPath, expectedDB)
	}

	expectedDir := filepath.Join(home, ".config/snip/filters")
	if cfg.Filters.Dir != expectedDir {
		t.Errorf("filters.dir: got %q, want %q", cfg.Filters.Dir, expectedDir)
	}
}

func TestExpandTildeNoTilde(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[tracking]
db_path = "/absolute/path/tracking.db"

[filters]
dir = "/absolute/path/filters"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Tracking.DBPath != "/absolute/path/tracking.db" {
		t.Errorf("db_path: got %q, want absolute path", cfg.Tracking.DBPath)
	}
	if cfg.Filters.Dir != "/absolute/path/filters" {
		t.Errorf("filters.dir: got %q, want absolute path", cfg.Filters.Dir)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[tracking]
db_path = "/custom/path.db"

[tee]
mode = "always"
max_files = 5
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracking.DBPath != "/custom/path.db" {
		t.Errorf("expected custom db path, got %q", cfg.Tracking.DBPath)
	}
	if cfg.Tee.Mode != "always" {
		t.Errorf("expected tee mode 'always', got %q", cfg.Tee.Mode)
	}
	if cfg.Tee.MaxFiles != 5 {
		t.Errorf("expected max_files 5, got %d", cfg.Tee.MaxFiles)
	}
}
