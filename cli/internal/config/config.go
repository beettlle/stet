// Package config provides Stet configuration with a defined load order:
// CLI flags > environment variables > repo config > global config > defaults.
//
// Paths:
//   - Repo: .review/config.toml (relative to repo root)
//   - Global: XDG config dir, e.g. ~/.config/stet/config.toml (see os.UserConfigDir)
//
// Environment variables (override config files when set):
//   - STET_MODEL, STET_OLLAMA_BASE_URL, STET_CONTEXT_LIMIT, STET_WARN_THRESHOLD,
//   - STET_TIMEOUT (Go duration string or integer seconds), STET_STATE_DIR, STET_WORKTREE_ROOT,
//   - STET_TEMPERATURE, STET_NUM_CTX (Ollama model runtime options; passed to /api/generate),
//   - STET_OPTIMIZER_SCRIPT (command to run for stet optimize; e.g. python3 scripts/optimize.py).
//   - STET_RAG_SYMBOL_MAX_DEFINITIONS, STET_RAG_SYMBOL_MAX_TOKENS (RAG-lite symbol lookup; Sub-phase 6.8).
package config

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	// Temperature and NumCtx are passed to Ollama /api/generate options (defaults: 0.2, 32768).
	Temperature     float64 `toml:"temperature"`
	NumCtx          int     `toml:"num_ctx"`
	OptimizerScript string `toml:"optimizer_script"` // Command for stet optimize (e.g. python3 scripts/optimize.py).
	// RAGSymbolMaxDefinitions is the max number of symbol definitions to inject into the prompt (0 = disable). Default 10.
	RAGSymbolMaxDefinitions int `toml:"rag_symbol_max_definitions"`
	// RAGSymbolMaxTokens caps the token size of the symbol-definitions block (0 = no cap). Default 0.
	RAGSymbolMaxTokens int `toml:"rag_symbol_max_tokens"`
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
	Temperature     *float64
	NumCtx          *int
	OptimizerScript        *string
	RAGSymbolMaxDefinitions *int
	RAGSymbolMaxTokens      *int
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
	_defaultTemperature          = 0.2
	_defaultNumCtx               = 32768
	_defaultRAGSymbolMaxDefs     = 10
	_defaultRAGSymbolMaxTokens   = 0
)

// errIntOverflow is returned when an int64 value does not fit in int (e.g. on 32-bit or huge TOML/env values).
var errIntOverflow = errors.New("value out of range for int")

// int64ToInt converts n to int. It returns an error if n is outside the range of int (e.g. overflow on 32-bit).
func int64ToInt(n int64) (int, error) {
	if n < int64(math.MinInt) || n > int64(math.MaxInt) {
		return 0, errIntOverflow
	}
	return int(n), nil
}

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
		Temperature:             _defaultTemperature,
		NumCtx:                  _defaultNumCtx,
		RAGSymbolMaxDefinitions: _defaultRAGSymbolMaxDefs,
		RAGSymbolMaxTokens:      _defaultRAGSymbolMaxTokens,
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
		Model            *string  `toml:"model"`
		OllamaBaseURL    *string  `toml:"ollama_base_url"`
		ContextLimit     *int64   `toml:"context_limit"`
		WarnThreshold    *float64 `toml:"warn_threshold"`
		Timeout          *string  `toml:"timeout"`
		StateDir         *string  `toml:"state_dir"`
		WorktreeRoot     *string  `toml:"worktree_root"`
		Temperature      *float64 `toml:"temperature"`
		NumCtx           *int64   `toml:"num_ctx"`
		OptimizerScript         *string `toml:"optimizer_script"`
		RAGSymbolMaxDefinitions *int64  `toml:"rag_symbol_max_definitions"`
		RAGSymbolMaxTokens      *int64  `toml:"rag_symbol_max_tokens"`
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
		v, err := int64ToInt(*file.ContextLimit)
		if err != nil {
			return fmt.Errorf("config %s context_limit value out of range: %w", path, err)
		}
		cfg.ContextLimit = v
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
	if file.Temperature != nil && *file.Temperature >= 0 && *file.Temperature <= 2 {
		cfg.Temperature = *file.Temperature
	}
	if file.NumCtx != nil && *file.NumCtx > 0 {
		v, err := int64ToInt(*file.NumCtx)
		if err != nil {
			return fmt.Errorf("config %s num_ctx value out of range: %w", path, err)
		}
		cfg.NumCtx = v
	} else if file.NumCtx != nil && *file.NumCtx == 0 {
		cfg.NumCtx = _defaultNumCtx
	}
	if file.OptimizerScript != nil {
		cfg.OptimizerScript = *file.OptimizerScript
	}
	if file.RAGSymbolMaxDefinitions != nil && *file.RAGSymbolMaxDefinitions >= 0 {
		v, err := int64ToInt(*file.RAGSymbolMaxDefinitions)
		if err != nil {
			return fmt.Errorf("config %s rag_symbol_max_definitions value out of range: %w", path, err)
		}
		cfg.RAGSymbolMaxDefinitions = v
	}
	if file.RAGSymbolMaxTokens != nil && *file.RAGSymbolMaxTokens >= 0 {
		v, err := int64ToInt(*file.RAGSymbolMaxTokens)
		if err != nil {
			return fmt.Errorf("config %s rag_symbol_max_tokens value out of range: %w", path, err)
		}
		cfg.RAGSymbolMaxTokens = v
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
	envTemperature     = "STET_TEMPERATURE"
	envNumCtx          = "STET_NUM_CTX"
	envOptimizerScript         = "STET_OPTIMIZER_SCRIPT"
	envRAGSymbolMaxDefinitions = "STET_RAG_SYMBOL_MAX_DEFINITIONS"
	envRAGSymbolMaxTokens      = "STET_RAG_SYMBOL_MAX_TOKENS"
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
		cfg.ContextLimit, err = int64ToInt(n)
		if err != nil {
			return fmt.Errorf("%s: value out of range for int: %w", envContextLimit, err)
		}
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
	if v, ok := vals[envTemperature]; ok && v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("%s: invalid float %q: %w", envTemperature, v, err)
		}
		if f < 0 || f > 2 {
			return fmt.Errorf("%s: temperature must be in [0, 2], got %g", envTemperature, f)
		}
		cfg.Temperature = f
	}
	if v, ok := vals[envNumCtx]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("%s: invalid integer %q: %w", envNumCtx, v, err)
		}
		if n < 0 {
			return fmt.Errorf("%s: num_ctx must be non-negative, got %d", envNumCtx, n)
		}
		if n == 0 {
			cfg.NumCtx = _defaultNumCtx
		} else {
			cfg.NumCtx, err = int64ToInt(n)
			if err != nil {
				return fmt.Errorf("%s: value out of range for int: %w", envNumCtx, err)
			}
		}
	}
	if v, ok := vals[envOptimizerScript]; ok {
		cfg.OptimizerScript = v
	}
	if v, ok := vals[envRAGSymbolMaxDefinitions]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("%s: invalid integer %q: %w", envRAGSymbolMaxDefinitions, v, err)
		}
		if n >= 0 {
			cfg.RAGSymbolMaxDefinitions, err = int64ToInt(n)
			if err != nil {
				return fmt.Errorf("%s: value out of range for int: %w", envRAGSymbolMaxDefinitions, err)
			}
		}
	}
	if v, ok := vals[envRAGSymbolMaxTokens]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("%s: invalid integer %q: %w", envRAGSymbolMaxTokens, v, err)
		}
		if n >= 0 {
			cfg.RAGSymbolMaxTokens, err = int64ToInt(n)
			if err != nil {
				return fmt.Errorf("%s: value out of range for int: %w", envRAGSymbolMaxTokens, err)
			}
		}
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
	if o.Temperature != nil {
		cfg.Temperature = *o.Temperature
	}
	if o.NumCtx != nil {
		cfg.NumCtx = *o.NumCtx
	}
	if o.OptimizerScript != nil {
		cfg.OptimizerScript = *o.OptimizerScript
	}
	if o.RAGSymbolMaxDefinitions != nil {
		cfg.RAGSymbolMaxDefinitions = *o.RAGSymbolMaxDefinitions
	}
	if o.RAGSymbolMaxTokens != nil {
		cfg.RAGSymbolMaxTokens = *o.RAGSymbolMaxTokens
	}
}
