package config

import (
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

type Config struct {
	Tracking TrackingConfig `toml:"tracking"`
	Display  DisplayConfig  `toml:"display"`
	Filters  FiltersConfig  `toml:"filters"`
	Tee      TeeConfig      `toml:"tee"`
}

type TrackingConfig struct {
	DBPath string `toml:"db_path"`
}

type DisplayConfig struct {
	Color         bool `toml:"color"`
	Emoji         bool `toml:"emoji"`
	QuietNoFilter bool `toml:"quiet_no_filter"`
}

type FiltersConfig struct {
	Dir    string          `toml:"dir"`
	Enable map[string]bool `toml:"enable"`
}

type TeeConfig struct {
	Enabled     bool   `toml:"enabled"`
	Mode        string `toml:"mode"` // "failures", "always", "never"
	MaxFiles    int    `toml:"max_files"`
	MaxFileSize int64  `toml:"max_file_size"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &Config{
		Tracking: TrackingConfig{
			DBPath: filepath.Join(home, ".local", "share", "snip", "tracking.db"),
		},
		Display: DisplayConfig{
			Color: true,
			Emoji: true,
		},
		Filters: FiltersConfig{
			Dir: filepath.Join(home, ".config", "snip", "filters"),
		},
		Tee: TeeConfig{
			Enabled:     true,
			Mode:        "failures",
			MaxFiles:    20,
			MaxFileSize: 1 << 20, // 1MB
		},
	}
}

// Load reads config from file, merging with defaults. Returns defaults if file missing.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.expandTilde()

	return cfg, nil
}

// expandTilde replaces a leading "~/" with the user's home directory
// in all path fields. Go does not expand tildes from config files.
func (c *Config) expandTilde() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	c.Tracking.DBPath = expandPath(c.Tracking.DBPath, home)
	c.Filters.Dir = expandPath(c.Filters.Dir, home)
}

func expandPath(p, home string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

func configPath() string {
	if p := os.Getenv("SNIP_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "snip", "config.toml")
}
