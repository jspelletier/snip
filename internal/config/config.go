package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

var envVarRe = regexp.MustCompile(`\$\{env\.(\w+)\}`)

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
	Dir    any             `toml:"dir"`
	Enable map[string]bool `toml:"enable"`
}

// Dirs returns the filter directories as a normalized string slice.
// Dir can be a single string or an array of strings in TOML.
func (f *FiltersConfig) Dirs() []string {
	switch v := f.Dir.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []any:
		dirs := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				dirs = append(dirs, s)
			}
		}
		return dirs
	case []string:
		return v
	default:
		return nil
	}
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
		// go-toml/v2 cannot decode a TOML array into interface{}.
		// Retry with an alternative struct that accepts dir as []string.
		cfg = DefaultConfig()
		if !tryUnmarshalArrayDir(data, cfg) {
			return nil, err
		}
	}

	cfg.expandPaths()

	return cfg, nil
}

// tryUnmarshalArrayDir handles the case where filters.dir is a TOML array.
func tryUnmarshalArrayDir(data []byte, cfg *Config) bool {
	type filtersArray struct {
		Dir    []string        `toml:"dir"`
		Enable map[string]bool `toml:"enable"`
	}
	type configArray struct {
		Tracking TrackingConfig `toml:"tracking"`
		Display  DisplayConfig  `toml:"display"`
		Filters  filtersArray   `toml:"filters"`
		Tee      TeeConfig      `toml:"tee"`
	}

	def := DefaultConfig()
	alt := configArray{
		Tracking: def.Tracking,
		Display:  def.Display,
		Filters:  filtersArray{Dir: def.Filters.Dirs()},
		Tee:      def.Tee,
	}

	if err := toml.Unmarshal(data, &alt); err != nil {
		return false
	}

	cfg.Tracking = alt.Tracking
	cfg.Display = alt.Display
	cfg.Filters.Dir = alt.Filters.Dir
	cfg.Filters.Enable = alt.Filters.Enable
	cfg.Tee = alt.Tee
	return true
}

// expandPaths expands ${env.VAR} references and leading "~/" in all path fields.
func (c *Config) expandPaths() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	c.Tracking.DBPath = expandPath(expandEnvVars(c.Tracking.DBPath), home)

	dirs := c.Filters.Dirs()
	expanded := make([]string, len(dirs))
	for i, d := range dirs {
		expanded[i] = expandPath(expandEnvVars(d), home)
	}
	c.Filters.Dir = expanded
}

func expandPath(p, home string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// expandEnvVars replaces ${env.VAR} patterns with the corresponding
// environment variable value.
func expandEnvVars(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		// "${env.VAR}" -> extract "VAR"
		name := match[6 : len(match)-1]
		return os.Getenv(name)
	})
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
