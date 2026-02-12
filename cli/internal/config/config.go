// Package config provides Stet configuration with a defined load order:
// CLI flags > environment variables > repo config > global config > defaults.
//
// Paths:
//   - Repo: .review/config.toml (relative to repo root)
//   - Global: XDG config dir, e.g. ~/.config/stet/config.toml (see os.UserConfigDir)
//
// Environment variables (override config files when set):
//   - STET_MODEL, STET_OLLAMA_BASE_URL, STET_CONTEXT_LIMIT, STET_WARN_THRESHOLD,
//   - STET_TIMEOUT (Go duration string or integer seconds), STET_STATE_DIR, STET_WORKTREE_ROOT
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config holds all Stet configuration. Empty string or zero values for
// StateDir/WorktreeRoot mean "use default behavior" (e.g. .review in repo).
type Config struct {
	Model         string        `toml:"model"`
	OllamaBaseURL string        `toml:"ollama_base_url"`
	ContextLimit  int           `toml:"context_limit"`
	WarnThreshold float64       `toml:"warn_threshold"`
	Timeout       time.Duration `toml:"timeout"`
	StateDir      string        `toml:"state_dir"`
	WorktreeRoot  string        `toml:"worktree_root"`
}

// Overrides represents optional CLI flag overrides. Non-nil pointer means
// "override with this value". Used by future Cobra flags; tests pass explicitly.
type Overrides struct {
	Model         *string
	OllamaBaseURL *string
	ContextLimit  *int
	WarnThreshold *float64
	Timeout       *time.Duration
	StateDir      *string
	WorktreeRoot  *string
}

// LoadOptions configures Load. All fields are optional.
type LoadOptions struct {
	// RepoRoot is the repository root; if set, repo config is RepoRoot/.review/config.toml.
	RepoRoot string
	// GlobalConfigPath is the global config file path; if empty, XDG path is used.
	GlobalConfigPath string
	// Env is the environment key=value slice; if nil, os.Environ() is used.
	Env []string
	// Overrides are applied last (highest precedence).
	Overrides *Overrides
}

const (
	_defaultModel         = "qwen3-coder:30b"
	_defaultOllamaBaseURL = "http://localhost:11434"
	_defaultContextLimit  = 32768
	_defaultWarnThreshold = 0.9
	_defaultTimeout       = 5 * time.Minute
)

// DefaultConfig returns the default configuration (no I/O).
func DefaultConfig() Config {
	return Config{
		Model:         _defaultModel,
		OllamaBaseURL: _defaultOllamaBaseURL,
		ContextLimit:  _defaultContextLimit,
		WarnThreshold: _defaultWarnThreshold,
		Timeout:       _defaultTimeout,
		StateDir:      "",
		WorktreeRoot:  "",
	}
}

// EffectiveStateDir returns the directory used for session and lock files.
// If StateDir is set, it is returned as-is; otherwise repoRoot/.review is returned.
func (c Config) EffectiveStateDir(repoRoot string) string {
	if c.StateDir != "" {
		return c.StateDir
	}
	return filepath.Join(repoRoot, ".review")
}

// Load loads configuration with precedence: defaults < global file < repo file < env < overrides.
// Missing config files are ignored. Invalid TOML or invalid env values return an error.
func Load(ctx context.Context, opts LoadOptions) (*Config, error) {
	if opts.Env == nil {
		opts.Env = os.Environ()
	}
	cfg := DefaultConfig()

	globalPath := opts.GlobalConfigPath
	if globalPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("global config path: %w", err)
		}
		globalPath = filepath.Join(dir, "stet", "config.toml")
	}
	if err := mergeFile(&cfg, globalPath); err != nil {
		return nil, err
	}

	if opts.RepoRoot != "" {
		repoPath := filepath.Join(opts.RepoRoot, ".review", "config.toml")
		if err := mergeFile(&cfg, repoPath); err != nil {
			return nil, err
		}
	}

	if err := applyEnv(&cfg, opts.Env); err != nil {
		return nil, err
	}

	applyOverrides(&cfg, opts.Overrides)
	return &cfg, nil
}

// mergeFile reads path and merges into cfg. Only overwrites fields that are
// present and non-zero in the file (so explicit empty/zero in TOML keeps previous value).
// Missing file or unreadable path is skipped (no error).
func mergeFile(cfg *Config, path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("config file %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	var file struct {
		Model         *string  `toml:"model"`
		OllamaBaseURL *string  `toml:"ollama_base_url"`
		ContextLimit  *int64   `toml:"context_limit"`
		WarnThreshold *float64 `toml:"warn_threshold"`
		Timeout       *string  `toml:"timeout"`
		StateDir      *string  `toml:"state_dir"`
		WorktreeRoot  *string  `toml:"worktree_root"`
	}
	if _, err := toml.Decode(string(data), &file); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	if file.Model != nil && *file.Model != "" {
		cfg.Model = *file.Model
	}
	if file.OllamaBaseURL != nil && *file.OllamaBaseURL != "" {
		cfg.OllamaBaseURL = *file.OllamaBaseURL
	}
	if file.ContextLimit != nil && *file.ContextLimit > 0 {
		cfg.ContextLimit = int(*file.ContextLimit)
	}
	if file.WarnThreshold != nil && *file.WarnThreshold >= 0 {
		cfg.WarnThreshold = *file.WarnThreshold
	}
	if file.Timeout != nil && *file.Timeout != "" {
		d, err := parseDuration(*file.Timeout)
		if err != nil {
			return fmt.Errorf("config %s timeout: %w", path, err)
		}
		cfg.Timeout = d
	}
	if file.StateDir != nil {
		cfg.StateDir = *file.StateDir
	}
	if file.WorktreeRoot != nil {
		cfg.WorktreeRoot = *file.WorktreeRoot
	}
	return nil
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	// Try Go duration first (e.g. "5m", "30s")
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}
	// Try integer seconds
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return time.Duration(n) * time.Second, nil
}

// env key names for config
const (
	envModel         = "STET_MODEL"
	envOllamaBaseURL = "STET_OLLAMA_BASE_URL"
	envContextLimit  = "STET_CONTEXT_LIMIT"
	envWarnThreshold = "STET_WARN_THRESHOLD"
	envTimeout       = "STET_TIMEOUT"
	envStateDir      = "STET_STATE_DIR"
	envWorktreeRoot  = "STET_WORKTREE_ROOT"
)

func applyEnv(cfg *Config, env []string) error {
	vals := make(map[string]string)
	for _, e := range env {
		idx := strings.Index(e, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(e[:idx])
		val := strings.TrimSpace(e[idx+1:])
		vals[key] = val
	}
	if v, ok := vals[envModel]; ok && v != "" {
		cfg.Model = v
	}
	if v, ok := vals[envOllamaBaseURL]; ok && v != "" {
		cfg.OllamaBaseURL = v
	}
	if v, ok := vals[envContextLimit]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("%s: invalid integer %q: %w", envContextLimit, v, err)
		}
		cfg.ContextLimit = int(n)
	}
	if v, ok := vals[envWarnThreshold]; ok && v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("%s: invalid float %q: %w", envWarnThreshold, v, err)
		}
		cfg.WarnThreshold = f
	}
	if v, ok := vals[envTimeout]; ok && v != "" {
		d, err := parseDuration(v)
		if err != nil {
			return fmt.Errorf("%s: %w", envTimeout, err)
		}
		cfg.Timeout = d
	}
	if v, ok := vals[envStateDir]; ok {
		cfg.StateDir = v
	}
	if v, ok := vals[envWorktreeRoot]; ok {
		cfg.WorktreeRoot = v
	}
	return nil
}

func applyOverrides(cfg *Config, o *Overrides) {
	if o == nil {
		return
	}
	if o.Model != nil {
		cfg.Model = *o.Model
	}
	if o.OllamaBaseURL != nil {
		cfg.OllamaBaseURL = *o.OllamaBaseURL
	}
	if o.ContextLimit != nil {
		cfg.ContextLimit = *o.ContextLimit
	}
	if o.WarnThreshold != nil {
		cfg.WarnThreshold = *o.WarnThreshold
	}
	if o.Timeout != nil {
		cfg.Timeout = *o.Timeout
	}
	if o.StateDir != nil {
		cfg.StateDir = *o.StateDir
	}
	if o.WorktreeRoot != nil {
		cfg.WorktreeRoot = *o.WorktreeRoot
	}
}
