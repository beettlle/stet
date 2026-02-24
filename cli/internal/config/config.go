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
//   - STET_RAG_CALL_GRAPH_ENABLED, STET_RAG_CALLERS_MAX, STET_RAG_CALLEES_MAX, STET_RAG_CALL_GRAPH_MAX_TOKENS (RAG call-graph for Go).
//   - STET_STRICTNESS (review strictness preset: strict, default, lenient, strict+, default+, lenient+).
//   - STET_NITPICKY (enable nitpicky mode: 1/true/yes/on = true, 0/false/no/off = false).
//   - STET_SUPPRESSION_ENABLED (history-based suppression: 1/true/yes/on = true, 0/false/no/off = false).
//   - STET_SUPPRESSION_HISTORY_COUNT (max history records to scan for dismissals; non-negative integer).
//   - STET_CRITIC_ENABLED (optional second-pass critic: 1/true/yes/on = true, 0/false/no/off = false).
//   - STET_CRITIC_MODEL (model name for the critic, e.g. qwen2.5-coder:3b).
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

	"stet/cli/internal/erruser"
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
	// RAGCallGraphEnabled enables call-graph (callers/callees) for Go hunks. Default false.
	RAGCallGraphEnabled bool `toml:"rag_call_graph_enabled"`
	// RAGCallersMax is the max number of call sites to include (0 = use default 3). Default 3.
	RAGCallersMax int `toml:"rag_callers_max"`
	// RAGCalleesMax is the max number of callees to include (0 = use default 3). Default 3.
	RAGCalleesMax int `toml:"rag_callees_max"`
	// RAGCallGraphMaxTokens caps the call-graph block size (0 = use fraction of RAG budget). Default 0.
	RAGCallGraphMaxTokens int `toml:"rag_call_graph_max_tokens"`
	// Strictness is the review preset: strict, default, lenient, strict+, default+, lenient+ (case-insensitive).
	Strictness string `toml:"strictness"`
	// Nitpicky enables convention- and typo-aware review; when true, FP kill list is not applied.
	Nitpicky bool `toml:"nitpicky"`
	// SuppressionEnabled enables history-based suppression (prompt injection of dismissed examples). Default true.
	SuppressionEnabled bool `toml:"suppression_enabled"`
	// SuppressionHistoryCount is the max number of history records to scan for dismissals (0 = do not use history). Default 50.
	SuppressionHistoryCount int `toml:"suppression_history_count"`
	// CriticEnabled runs a second LLM pass (critic) on each finding; when true, findings the critic rejects are dropped. Default false.
	CriticEnabled bool `toml:"critic_enabled"`
	// CriticModel is the model name for the critic (e.g. qwen2.5-coder:3b). Used only when CriticEnabled. Enabling critic increases latency and token usage.
	CriticModel string `toml:"critic_model"`
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
	RAGCallGraphEnabled     *bool
	RAGCallersMax           *int
	RAGCalleesMax           *int
	RAGCallGraphMaxTokens   *int
	Strictness              *string
	Nitpicky                *bool
	SuppressionEnabled       *bool
	SuppressionHistoryCount  *int
	CriticEnabled           *bool
	CriticModel             *string
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
	_defaultRAGSymbolMaxDefs       = 10
	_defaultRAGSymbolMaxTokens    = 0
	_defaultRAGCallGraphEnabled   = false
	_defaultRAGCallersMax         = 3
	_defaultRAGCalleesMax         = 3
	_defaultRAGCallGraphMaxTokens = 0
	_defaultStrictness             = "default"
	_defaultSuppressionHistoryCount = 50
	_defaultCriticModel            = "qwen2.5-coder:3b"
)

// validStrictness is the set of allowed strictness values (normalized lowercase).
var validStrictness = map[string]struct{}{
	"strict": {}, "default": {}, "lenient": {},
	"strict+": {}, "default+": {}, "lenient+": {},
}

// validateStrictness normalizes s (trim, lowercase) and returns it if valid; otherwise returns an error.
func validateStrictness(s string) (string, error) {
	norm := strings.TrimSpace(strings.ToLower(s))
	if _, ok := validStrictness[norm]; !ok {
		return "", erruser.New("Invalid strictness; use strict, default, lenient, strict+, default+, or lenient+.", nil)
	}
	return norm, nil
}

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
		RAGCallGraphEnabled:     _defaultRAGCallGraphEnabled,
		RAGCallersMax:           _defaultRAGCallersMax,
		RAGCalleesMax:           _defaultRAGCalleesMax,
		RAGCallGraphMaxTokens:   _defaultRAGCallGraphMaxTokens,
		Strictness:                _defaultStrictness,
		Nitpicky:                  false,
		SuppressionEnabled:        true,
		SuppressionHistoryCount:   _defaultSuppressionHistoryCount,
		CriticEnabled:             false,
		CriticModel:               _defaultCriticModel,
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
			return nil, erruser.New("Could not determine config directory.", err)
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
		return erruser.New("Invalid configuration file.", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return erruser.New("Could not read configuration file.", err)
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
		RAGCallGraphEnabled     *bool   `toml:"rag_call_graph_enabled"`
		RAGCallersMax           *int64  `toml:"rag_callers_max"`
		RAGCalleesMax           *int64  `toml:"rag_callees_max"`
		RAGCallGraphMaxTokens   *int64  `toml:"rag_call_graph_max_tokens"`
		Strictness               *string `toml:"strictness"`
		Nitpicky                 *bool   `toml:"nitpicky"`
		SuppressionEnabled       *bool   `toml:"suppression_enabled"`
		SuppressionHistoryCount  *int64  `toml:"suppression_history_count"`
		CriticEnabled            *bool   `toml:"critic_enabled"`
		CriticModel              *string `toml:"critic_model"`
	}
	if _, err := toml.Decode(string(data), &file); err != nil {
		return erruser.New("Invalid configuration in .review/config.toml.", err)
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
			return erruser.New("Configuration context_limit value out of range.", err)
		}
		cfg.ContextLimit = v
	}
	if file.WarnThreshold != nil && *file.WarnThreshold >= 0 {
		cfg.WarnThreshold = *file.WarnThreshold
	}
	if file.Timeout != nil && *file.Timeout != "" {
		d, err := parseDuration(*file.Timeout)
		if err != nil {
			return erruser.New("Configuration timeout is invalid.", err)
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
			return erruser.New("Configuration num_ctx value out of range.", err)
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
			return erruser.New("Configuration rag_symbol_max_definitions value out of range.", err)
		}
		cfg.RAGSymbolMaxDefinitions = v
	}
	if file.RAGSymbolMaxTokens != nil && *file.RAGSymbolMaxTokens >= 0 {
		v, err := int64ToInt(*file.RAGSymbolMaxTokens)
		if err != nil {
			return erruser.New("Configuration rag_symbol_max_tokens value out of range.", err)
		}
		cfg.RAGSymbolMaxTokens = v
	}
	if file.RAGCallGraphEnabled != nil {
		cfg.RAGCallGraphEnabled = *file.RAGCallGraphEnabled
	}
	if file.RAGCallersMax != nil && *file.RAGCallersMax >= 0 {
		v, err := int64ToInt(*file.RAGCallersMax)
		if err != nil {
			return erruser.New("Configuration rag_callers_max value out of range.", err)
		}
		cfg.RAGCallersMax = v
	}
	if file.RAGCalleesMax != nil && *file.RAGCalleesMax >= 0 {
		v, err := int64ToInt(*file.RAGCalleesMax)
		if err != nil {
			return erruser.New("Configuration rag_callees_max value out of range.", err)
		}
		cfg.RAGCalleesMax = v
	}
	if file.RAGCallGraphMaxTokens != nil && *file.RAGCallGraphMaxTokens >= 0 {
		v, err := int64ToInt(*file.RAGCallGraphMaxTokens)
		if err != nil {
			return erruser.New("Configuration rag_call_graph_max_tokens value out of range.", err)
		}
		cfg.RAGCallGraphMaxTokens = v
	}
	if file.Strictness != nil && *file.Strictness != "" {
		norm, err := validateStrictness(*file.Strictness)
		if err != nil {
			return err
		}
		cfg.Strictness = norm
	}
	if file.Nitpicky != nil {
		cfg.Nitpicky = *file.Nitpicky
	}
	if file.SuppressionEnabled != nil {
		cfg.SuppressionEnabled = *file.SuppressionEnabled
	}
	if file.SuppressionHistoryCount != nil && *file.SuppressionHistoryCount >= 0 {
		v, err := int64ToInt(*file.SuppressionHistoryCount)
		if err != nil {
			return erruser.New("Configuration suppression_history_count value out of range.", err)
		}
		cfg.SuppressionHistoryCount = v
	}
	if file.CriticEnabled != nil {
		cfg.CriticEnabled = *file.CriticEnabled
	}
	if file.CriticModel != nil && *file.CriticModel != "" {
		cfg.CriticModel = *file.CriticModel
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
	envRAGCallGraphEnabled     = "STET_RAG_CALL_GRAPH_ENABLED"
	envRAGCallersMax            = "STET_RAG_CALLERS_MAX"
	envRAGCalleesMax            = "STET_RAG_CALLEES_MAX"
	envRAGCallGraphMaxTokens    = "STET_RAG_CALL_GRAPH_MAX_TOKENS"
	envStrictness               = "STET_STRICTNESS"
	envNitpicky                 = "STET_NITPICKY"
	envSuppressionEnabled       = "STET_SUPPRESSION_ENABLED"
	envSuppressionHistoryCount  = "STET_SUPPRESSION_HISTORY_COUNT"
	envCriticEnabled            = "STET_CRITIC_ENABLED"
	envCriticModel              = "STET_CRITIC_MODEL"
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
			return erruser.New("STET_CONTEXT_LIMIT must be a valid number.", err)
		}
		cfg.ContextLimit, err = int64ToInt(n)
		if err != nil {
			return erruser.New("STET_CONTEXT_LIMIT value out of range.", err)
		}
	}
	if v, ok := vals[envWarnThreshold]; ok && v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return erruser.New("STET_WARN_THRESHOLD must be a valid number.", err)
		}
		cfg.WarnThreshold = f
	}
	if v, ok := vals[envTimeout]; ok && v != "" {
		d, err := parseDuration(v)
		if err != nil {
			return erruser.New("STET_TIMEOUT must be a valid duration.", err)
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
			return erruser.New("STET_TEMPERATURE must be a valid number.", err)
		}
		if f < 0 || f > 2 {
			return erruser.New("STET_TEMPERATURE must be between 0 and 2.", nil)
		}
		cfg.Temperature = f
	}
	if v, ok := vals[envNumCtx]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return erruser.New("STET_NUM_CTX must be a valid number.", err)
		}
		if n < 0 {
			return erruser.New("STET_NUM_CTX must be non-negative.", nil)
		}
		if n == 0 {
			cfg.NumCtx = _defaultNumCtx
		} else {
			cfg.NumCtx, err = int64ToInt(n)
			if err != nil {
				return erruser.New("STET_NUM_CTX value out of range.", err)
			}
		}
	}
	if v, ok := vals[envOptimizerScript]; ok {
		cfg.OptimizerScript = v
	}
	if v, ok := vals[envRAGSymbolMaxDefinitions]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return erruser.New("STET_RAG_SYMBOL_MAX_DEFINITIONS must be a valid number.", err)
		}
		if n >= 0 {
			cfg.RAGSymbolMaxDefinitions, err = int64ToInt(n)
			if err != nil {
				return erruser.New("STET_RAG_SYMBOL_MAX_DEFINITIONS value out of range.", err)
			}
		}
	}
	if v, ok := vals[envRAGSymbolMaxTokens]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return erruser.New("STET_RAG_SYMBOL_MAX_TOKENS must be a valid number.", err)
		}
		if n >= 0 {
			cfg.RAGSymbolMaxTokens, err = int64ToInt(n)
			if err != nil {
				return erruser.New("STET_RAG_SYMBOL_MAX_TOKENS value out of range.", err)
			}
		}
	}
	if v, ok := vals[envRAGCallGraphEnabled]; ok && v != "" {
		b, err := parseBool(v)
		if err != nil {
			return erruser.New("STET_RAG_CALL_GRAPH_ENABLED must be 1/true/yes/on or 0/false/no/off.", err)
		}
		cfg.RAGCallGraphEnabled = b
	}
	if v, ok := vals[envRAGCallersMax]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return erruser.New("STET_RAG_CALLERS_MAX must be a valid number.", err)
		}
		if n >= 0 {
			cfg.RAGCallersMax, err = int64ToInt(n)
			if err != nil {
				return erruser.New("STET_RAG_CALLERS_MAX value out of range.", err)
			}
		}
	}
	if v, ok := vals[envRAGCalleesMax]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return erruser.New("STET_RAG_CALLEES_MAX must be a valid number.", err)
		}
		if n >= 0 {
			cfg.RAGCalleesMax, err = int64ToInt(n)
			if err != nil {
				return erruser.New("STET_RAG_CALLEES_MAX value out of range.", err)
			}
		}
	}
	if v, ok := vals[envRAGCallGraphMaxTokens]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return erruser.New("STET_RAG_CALL_GRAPH_MAX_TOKENS must be a valid number.", err)
		}
		if n >= 0 {
			cfg.RAGCallGraphMaxTokens, err = int64ToInt(n)
			if err != nil {
				return erruser.New("STET_RAG_CALL_GRAPH_MAX_TOKENS value out of range.", err)
			}
		}
	}
	if v, ok := vals[envStrictness]; ok && v != "" {
		norm, err := validateStrictness(v)
		if err != nil {
			return err
		}
		cfg.Strictness = norm
	}
	if v, ok := vals[envNitpicky]; ok && v != "" {
		b, err := parseBool(v)
		if err != nil {
			return erruser.New("STET_NITPICKY must be 1/true/yes/on or 0/false/no/off.", err)
		}
		cfg.Nitpicky = b
	}
	if v, ok := vals[envSuppressionEnabled]; ok && v != "" {
		b, err := parseBool(v)
		if err != nil {
			return erruser.New("STET_SUPPRESSION_ENABLED must be 1/true/yes/on or 0/false/no/off.", err)
		}
		cfg.SuppressionEnabled = b
	}
	if v, ok := vals[envSuppressionHistoryCount]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return erruser.New("STET_SUPPRESSION_HISTORY_COUNT must be a valid number.", err)
		}
		if n < 0 {
			return erruser.New("STET_SUPPRESSION_HISTORY_COUNT must be non-negative.", nil)
		}
		cfg.SuppressionHistoryCount, err = int64ToInt(n)
		if err != nil {
			return erruser.New("STET_SUPPRESSION_HISTORY_COUNT value out of range.", err)
		}
	}
	if v, ok := vals[envCriticEnabled]; ok && v != "" {
		b, err := parseBool(v)
		if err != nil {
			return erruser.New("STET_CRITIC_ENABLED must be 1/true/yes/on or 0/false/no/off.", err)
		}
		cfg.CriticEnabled = b
	}
	if v, ok := vals[envCriticModel]; ok && v != "" {
		cfg.CriticModel = v
	}
	return nil
}

// parseBool parses common boolean env values: 1/true/yes/on = true, 0/false/no/off = false (case-insensitive).
func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", s)
	}
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
	if o.RAGCallGraphEnabled != nil {
		cfg.RAGCallGraphEnabled = *o.RAGCallGraphEnabled
	}
	if o.RAGCallersMax != nil {
		v := *o.RAGCallersMax
		if v < 0 {
			v = 0
		}
		cfg.RAGCallersMax = v
	}
	if o.RAGCalleesMax != nil {
		v := *o.RAGCalleesMax
		if v < 0 {
			v = 0
		}
		cfg.RAGCalleesMax = v
	}
	if o.RAGCallGraphMaxTokens != nil {
		v := *o.RAGCallGraphMaxTokens
		if v < 0 {
			v = 0
		}
		cfg.RAGCallGraphMaxTokens = v
	}
	if o.Strictness != nil && *o.Strictness != "" {
		if norm, err := validateStrictness(*o.Strictness); err == nil {
			cfg.Strictness = norm
		}
	}
	if o.Nitpicky != nil {
		cfg.Nitpicky = *o.Nitpicky
	}
	if o.SuppressionEnabled != nil {
		cfg.SuppressionEnabled = *o.SuppressionEnabled
	}
	if o.SuppressionHistoryCount != nil {
		v := *o.SuppressionHistoryCount
		if v < 0 {
			v = 0
		}
		cfg.SuppressionHistoryCount = v
	}
	if o.CriticEnabled != nil {
		cfg.CriticEnabled = *o.CriticEnabled
	}
	if o.CriticModel != nil && *o.CriticModel != "" {
		cfg.CriticModel = *o.CriticModel
	}
}
