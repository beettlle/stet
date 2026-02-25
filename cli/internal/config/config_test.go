package config

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func ptrStr(s string) *string { return &s }
func ptrInt(n int) *int       { return &n }
func ptrBool(b bool) *bool    { return &b }

func TestDefaultConfig_fieldsMatchExpectedDefaults(t *testing.T) {
	t.Parallel()
	c := DefaultConfig()
	if c.Model != _defaultModel {
		t.Errorf("Model = %q, want %q", c.Model, _defaultModel)
	}
	if c.OllamaBaseURL != _defaultOllamaBaseURL {
		t.Errorf("OllamaBaseURL = %q, want %q", c.OllamaBaseURL, _defaultOllamaBaseURL)
	}
	if c.ContextLimit != _defaultContextLimit {
		t.Errorf("ContextLimit = %d, want %d", c.ContextLimit, _defaultContextLimit)
	}
	if c.WarnThreshold != _defaultWarnThreshold {
		t.Errorf("WarnThreshold = %f, want %f", c.WarnThreshold, _defaultWarnThreshold)
	}
	if c.Timeout != _defaultTimeout {
		t.Errorf("Timeout = %v, want %v", c.Timeout, _defaultTimeout)
	}
	if c.Temperature != _defaultTemperature {
		t.Errorf("Temperature = %f, want %f", c.Temperature, _defaultTemperature)
	}
	if c.NumCtx != _defaultNumCtx {
		t.Errorf("NumCtx = %d, want %d", c.NumCtx, _defaultNumCtx)
	}
	if c.StateDir != "" || c.WorktreeRoot != "" {
		t.Errorf("StateDir or WorktreeRoot non-empty: %q, %q", c.StateDir, c.WorktreeRoot)
	}
	if c.Strictness != _defaultStrictness {
		t.Errorf("Strictness = %q, want %q", c.Strictness, _defaultStrictness)
	}
	if !c.SuppressionEnabled {
		t.Errorf("SuppressionEnabled = false, want true (default)")
	}
	if c.SuppressionHistoryCount != _defaultSuppressionHistoryCount {
		t.Errorf("SuppressionHistoryCount = %d, want %d", c.SuppressionHistoryCount, _defaultSuppressionHistoryCount)
	}
}

func TestLoad_defaultsOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nonexistent.toml"),
		Env:              []string{},
		Overrides:        nil,
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := DefaultConfig()
	if cfg.Model != want.Model || cfg.OllamaBaseURL != want.OllamaBaseURL ||
		cfg.ContextLimit != want.ContextLimit || cfg.WarnThreshold != want.WarnThreshold ||
		cfg.Timeout != want.Timeout || cfg.Temperature != want.Temperature || cfg.NumCtx != want.NumCtx {
		t.Errorf("got %+v, want defaults %+v", cfg, want)
	}
}

func TestLoad_globalOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global.toml")
	globalDir := filepath.Dir(globalPath)
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte(`model = "custom-model"`), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		GlobalConfigPath: globalPath,
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "custom-model" {
		t.Errorf("Model = %q, want custom-model", cfg.Model)
	}
	if cfg.OllamaBaseURL != _defaultOllamaBaseURL {
		t.Errorf("OllamaBaseURL should remain default, got %q", cfg.OllamaBaseURL)
	}
}

func TestLoad_repoOverridesGlobal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global.toml")
	repoRoot := filepath.Join(dir, "repo")
	repoDir := filepath.Join(repoRoot, ".review")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	repoPath := filepath.Join(repoDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(`model = "global-model"`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repoPath, []byte(`model = "repo-model"`), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         repoRoot,
		GlobalConfigPath: globalPath,
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "repo-model" {
		t.Errorf("Model = %q, want repo-model (repo overrides global)", cfg.Model)
	}
}

func TestLoad_envOverridesRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoRoot := filepath.Join(dir, "repo")
	repoDir := filepath.Join(repoRoot, ".review")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	repoPath := filepath.Join(repoDir, "config.toml")
	if err := os.WriteFile(repoPath, []byte(`model = "repo-model"`), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         repoRoot,
		GlobalConfigPath: filepath.Join(dir, "nonexistent.toml"),
		Env:              []string{"STET_MODEL=env-model"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "env-model" {
		t.Errorf("Model = %q, want env-model", cfg.Model)
	}
}

func TestLoad_overridesOverrideEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_MODEL=env-model"},
		Overrides:        &Overrides{Model: ptrStr("flag-model")},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "flag-model" {
		t.Errorf("Model = %q, want flag-model", cfg.Model)
	}
}

func TestLoad_precedenceChain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global.toml")
	repoRoot := filepath.Join(dir, "repo")
	repoDir := filepath.Join(repoRoot, ".review")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	repoPath := filepath.Join(repoDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte("model = \"global\"\nollama_base_url = \"http://global:11434\""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repoPath, []byte("model = \"repo\"\nollama_base_url = \"http://repo:11434\""), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         repoRoot,
		GlobalConfigPath: globalPath,
		Env:              []string{"STET_MODEL=env-model", "STET_OLLAMA_BASE_URL=http://env:11434"},
		Overrides:        &Overrides{Model: ptrStr("override-model"), OllamaBaseURL: ptrStr("http://override:11434")},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "override-model" {
		t.Errorf("Model = %q, want override-model", cfg.Model)
	}
	if cfg.OllamaBaseURL != "http://override:11434" {
		t.Errorf("OllamaBaseURL = %q, want http://override:11434", cfg.OllamaBaseURL)
	}
}

func TestLoad_overridesRAGOptions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	defs, tok := 5, 500
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{},
		Overrides:        &Overrides{RAGSymbolMaxDefinitions: ptrInt(defs), RAGSymbolMaxTokens: ptrInt(tok)},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.RAGSymbolMaxDefinitions != defs {
		t.Errorf("RAGSymbolMaxDefinitions = %d, want %d", cfg.RAGSymbolMaxDefinitions, defs)
	}
	if cfg.RAGSymbolMaxTokens != tok {
		t.Errorf("RAGSymbolMaxTokens = %d, want %d", cfg.RAGSymbolMaxTokens, tok)
	}
}

func TestLoad_invalidTOML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(badPath, []byte("model = [ broken"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		GlobalConfigPath: badPath,
		Env:              []string{},
	})
	if err == nil {
		t.Fatal("Load: expected error for invalid TOML")
	}
}

func TestLoad_invalidEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_CONTEXT_LIMIT=nan"},
	})
	if err == nil {
		t.Fatal("Load: expected error for STET_CONTEXT_LIMIT=nan")
	}
}

func TestLoad_invalidTimeoutEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_TIMEOUT=not-a-duration"},
	})
	if err == nil {
		t.Fatal("Load: expected error for invalid STET_TIMEOUT")
	}
}

func TestLoad_missingFilesNoError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "missing.toml"),
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v (missing files should be ok)", err)
	}
	if cfg.Model != _defaultModel {
		t.Errorf("Model = %q, want default", cfg.Model)
	}
}

func TestLoad_timeoutInTOML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global.toml")
	if err := os.WriteFile(globalPath, []byte(`timeout = "2m"`), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		GlobalConfigPath: globalPath,
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := 2 * time.Minute
	if cfg.Timeout != want {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, want)
	}
}

func TestLoad_envTimeoutSeconds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_TIMEOUT=90"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Timeout != 90*time.Second {
		t.Errorf("Timeout = %v, want 90s", cfg.Timeout)
	}
}

func TestLoad_envTimeoutDurationString(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_TIMEOUT=3m"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Timeout != 3*time.Minute {
		t.Errorf("Timeout = %v, want 3m", cfg.Timeout)
	}
}

func TestLoad_contextLimitAndWarnThresholdFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_CONTEXT_LIMIT=16384", "STET_WARN_THRESHOLD=0.8"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ContextLimit != 16384 {
		t.Errorf("ContextLimit = %d, want 16384", cfg.ContextLimit)
	}
	if cfg.WarnThreshold != 0.8 {
		t.Errorf("WarnThreshold = %f, want 0.8", cfg.WarnThreshold)
	}
}

func TestLoad_stateDirAndWorktreeRootFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_STATE_DIR=/tmp/stet", "STET_WORKTREE_ROOT=/tmp/wt"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.StateDir != "/tmp/stet" || cfg.WorktreeRoot != "/tmp/wt" {
		t.Errorf("StateDir=%q WorktreeRoot=%q", cfg.StateDir, cfg.WorktreeRoot)
	}
}

func TestLoad_temperatureAndNumCtxFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_TEMPERATURE=0.5", "STET_NUM_CTX=8192"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Temperature != 0.5 {
		t.Errorf("Temperature = %f, want 0.5", cfg.Temperature)
	}
	if cfg.NumCtx != 8192 {
		t.Errorf("NumCtx = %d, want 8192", cfg.NumCtx)
	}
}

func TestLoad_temperatureAndNumCtxFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global.toml")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte("temperature = 0.1\nnum_ctx = 16384\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		GlobalConfigPath: globalPath,
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Temperature != 0.1 {
		t.Errorf("Temperature = %f, want 0.1", cfg.Temperature)
	}
	if cfg.NumCtx != 16384 {
		t.Errorf("NumCtx = %d, want 16384", cfg.NumCtx)
	}
}

func TestLoad_temperatureInvalidFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_TEMPERATURE=3"},
	})
	if err == nil {
		t.Fatal("Load: want error for temperature > 2, got nil")
	}
}

func TestLoad_numCtxZeroFromEnv_usesDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_NUM_CTX=0"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.NumCtx != _defaultNumCtx {
		t.Errorf("NumCtx = %d, want default %d", cfg.NumCtx, _defaultNumCtx)
	}
}

func TestEffectiveStateDir(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		cfg      Config
		repoRoot string
		want     string
	}{
		{
			name:     "empty StateDir returns repo_root/.review",
			cfg:      Config{StateDir: ""},
			repoRoot: "/home/user/repo",
			want:     filepath.Join("/home", "user", "repo", ".review"),
		},
		{
			name:     "non-empty StateDir returned as-is",
			cfg:      Config{StateDir: "/tmp/stet-state"},
			repoRoot: "/home/user/repo",
			want:     "/tmp/stet-state",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.cfg.EffectiveStateDir(tt.repoRoot)
			if got != tt.want {
				t.Errorf("EffectiveStateDir(%q) = %q, want %q", tt.repoRoot, got, tt.want)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"5m", 5 * time.Minute},
		{"30s", 30 * time.Second},
		{"90", 90 * time.Second},
		{"0", 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := parseDuration(tt.in)
			if err != nil {
				t.Fatalf("parseDuration(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseDuration_invalid(t *testing.T) {
	t.Parallel()
	_, err := parseDuration("")
	if err == nil {
		t.Error("expected error for empty duration")
	}
	_, err = parseDuration("x1m")
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestInt64ToInt_inRange(t *testing.T) {
	t.Parallel()
	for _, n := range []int64{0, 1, 100, int64(math.MaxInt) - 1, int64(math.MaxInt)} {
		got, err := int64ToInt(n)
		if err != nil {
			t.Errorf("int64ToInt(%d): %v", n, err)
			continue
		}
		if got != int(n) {
			t.Errorf("int64ToInt(%d) = %d, want %d", n, got, int(n))
		}
	}
	if math.MinInt == math.MinInt64 {
		got, err := int64ToInt(math.MinInt64)
		if err != nil {
			t.Errorf("int64ToInt(MinInt64) on 64-bit: %v", err)
		}
		if got != math.MinInt {
			t.Errorf("int64ToInt(MinInt64) = %d, want %d", got, math.MinInt)
		}
	}
}

func TestInt64ToInt_overflow(t *testing.T) {
	t.Parallel()
	// 2147483648 fits in int64 but exceeds int on 32-bit.
	v := int64(2147483648)
	got, err := int64ToInt(v)
	if math.MaxInt < 2147483648 {
		if err == nil {
			t.Errorf("int64ToInt(2147483648) on 32-bit: want error, got %d", got)
		}
		if err != nil && !errors.Is(err, errIntOverflow) {
			t.Errorf("int64ToInt: want errIntOverflow, got %v", err)
		}
	} else {
		if err != nil {
			t.Errorf("int64ToInt(2147483648) on 64-bit: %v", err)
		}
		if got != 2147483648 {
			t.Errorf("got %d, want 2147483648", got)
		}
	}
}

func TestLoad_tomlIntOverflow(t *testing.T) {
	t.Parallel()
	if math.MaxInt >= 2147483648 {
		t.Skip("int is 64-bit; no int64 value overflows int")
	}
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global.toml")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte("context_limit = 2147483648\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		GlobalConfigPath: globalPath,
		Env:              []string{},
	})
	if err == nil {
		t.Fatal("Load: want error for context_limit overflow on 32-bit, got nil")
	}
	if !errors.Is(err, errIntOverflow) && !strings.Contains(err.Error(), "out of range") {
		t.Errorf("Load: want out-of-range error, got %v", err)
	}
}

func TestLoad_envIntOverflow(t *testing.T) {
	t.Parallel()
	if math.MaxInt >= 2147483648 {
		t.Skip("int is 64-bit; no int64 value overflows int")
	}
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_CONTEXT_LIMIT=2147483648"},
	})
	if err == nil {
		t.Fatal("Load: want error for STET_CONTEXT_LIMIT overflow on 32-bit, got nil")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("Load: want out-of-range error, got %v", err)
	}
}

func TestLoad_strictnessFromTOML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".review"), 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(repoDir, ".review", "config.toml")
	if err := os.WriteFile(configPath, []byte(`strictness = "strict"`), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         repoDir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Strictness != "strict" {
		t.Errorf("Strictness = %q, want strict", cfg.Strictness)
	}
}

func TestLoad_strictnessFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_STRICTNESS=lenient+"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Strictness != "lenient+" {
		t.Errorf("Strictness = %q, want lenient+", cfg.Strictness)
	}
}

func TestLoad_strictnessInvalid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".review"), 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(repoDir, ".review", "config.toml")
	if err := os.WriteFile(configPath, []byte(`strictness = "aggressive"`), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		RepoRoot:         repoDir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{},
	})
	if err == nil {
		t.Fatal("Load: want error for invalid strictness, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid strictness") {
		t.Errorf("Load: want Invalid strictness in error, got %v", err)
	}
}

func TestLoad_strictnessFromOverrides(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{},
		Overrides:        &Overrides{Strictness: ptrStr("default+")},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Strictness != "default+" {
		t.Errorf("Strictness = %q, want default+", cfg.Strictness)
	}
}

func TestLoad_strictnessInvalidFromOverrides_keepsDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{},
		Overrides:        &Overrides{Strictness: ptrStr("invalid")},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Strictness != _defaultStrictness {
		t.Errorf("Strictness = %q, want default (invalid override ignored)", cfg.Strictness)
	}
}

func TestLoad_strictnessInvalidFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_STRICTNESS=aggressive"},
	})
	if err == nil {
		t.Fatal("Load: want error for invalid strictness from env, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid strictness") {
		t.Errorf("Load: want Invalid strictness in error, got %v", err)
	}
}

func TestLoad_strictnessNormalizedFromTOML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".review"), 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(repoDir, ".review", "config.toml")
	if err := os.WriteFile(configPath, []byte(`strictness = "  LENIENT+  "`), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         repoDir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Strictness != "lenient+" {
		t.Errorf("Strictness = %q, want lenient+ (normalized)", cfg.Strictness)
	}
}

func TestLoad_nitpickyDefaultFalse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Nitpicky {
		t.Errorf("Nitpicky = true, want false (default)")
	}
}

func TestLoad_nitpickyFromTOML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".review"), 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(repoDir, ".review", "config.toml")
	if err := os.WriteFile(configPath, []byte(`nitpicky = true`), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         repoDir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Nitpicky {
		t.Errorf("Nitpicky = false, want true")
	}
}

func TestLoad_nitpickyFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	for _, envVal := range []string{"1", "true", "yes", "on"} {
		cfg, err := Load(ctx, LoadOptions{
			RepoRoot:         dir,
			GlobalConfigPath: filepath.Join(dir, "nope.toml"),
			Env:              []string{"STET_NITPICKY=" + envVal},
		})
		if err != nil {
			t.Fatalf("Load(STET_NITPICKY=%s): %v", envVal, err)
		}
		if !cfg.Nitpicky {
			t.Errorf("Nitpicky = false for STET_NITPICKY=%s, want true", envVal)
		}
	}
	for _, envVal := range []string{"0", "false", "no", "off"} {
		cfg, err := Load(ctx, LoadOptions{
			RepoRoot:         dir,
			GlobalConfigPath: filepath.Join(dir, "nope.toml"),
			Env:              []string{"STET_NITPICKY=" + envVal},
		})
		if err != nil {
			t.Fatalf("Load(STET_NITPICKY=%s): %v", envVal, err)
		}
		if cfg.Nitpicky {
			t.Errorf("Nitpicky = true for STET_NITPICKY=%s, want false", envVal)
		}
	}
}

func TestLoad_nitpickyFromOverrides(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{},
		Overrides:        &Overrides{Nitpicky: ptrBool(true)},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Nitpicky {
		t.Errorf("Nitpicky = false, want true (from overrides)")
	}
}

func TestLoad_nitpickyInvalidFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_NITPICKY=invalid"},
	})
	if err == nil {
		t.Fatal("Load: want error for invalid STET_NITPICKY, got nil")
	}
	if !strings.Contains(err.Error(), "STET_NITPICKY") {
		t.Errorf("Load: want STET_NITPICKY in error, got %v", err)
	}
}

func TestLoad_suppressionEnabledFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	for _, envVal := range []string{"1", "true", "yes", "on"} {
		cfg, err := Load(ctx, LoadOptions{
			RepoRoot:         dir,
			GlobalConfigPath: filepath.Join(dir, "nope.toml"),
			Env:              []string{"STET_SUPPRESSION_ENABLED=" + envVal},
		})
		if err != nil {
			t.Fatalf("Load(STET_SUPPRESSION_ENABLED=%s): %v", envVal, err)
		}
		if !cfg.SuppressionEnabled {
			t.Errorf("SuppressionEnabled = false for STET_SUPPRESSION_ENABLED=%s, want true", envVal)
		}
	}
	for _, envVal := range []string{"0", "false", "no", "off"} {
		cfg, err := Load(ctx, LoadOptions{
			RepoRoot:         dir,
			GlobalConfigPath: filepath.Join(dir, "nope.toml"),
			Env:              []string{"STET_SUPPRESSION_ENABLED=" + envVal},
		})
		if err != nil {
			t.Fatalf("Load(STET_SUPPRESSION_ENABLED=%s): %v", envVal, err)
		}
		if cfg.SuppressionEnabled {
			t.Errorf("SuppressionEnabled = true for STET_SUPPRESSION_ENABLED=%s, want false", envVal)
		}
	}
}

func TestLoad_suppressionEnabledInvalidFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_SUPPRESSION_ENABLED=invalid"},
	})
	if err == nil {
		t.Fatal("Load: want error for invalid STET_SUPPRESSION_ENABLED, got nil")
	}
	if !strings.Contains(err.Error(), "STET_SUPPRESSION_ENABLED") {
		t.Errorf("Load: want STET_SUPPRESSION_ENABLED in error, got %v", err)
	}
}

func TestLoad_suppressionHistoryCountFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_SUPPRESSION_HISTORY_COUNT=50"},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SuppressionHistoryCount != 50 {
		t.Errorf("SuppressionHistoryCount = %d, want 50", cfg.SuppressionHistoryCount)
	}
	cfg, err = Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_SUPPRESSION_HISTORY_COUNT=0"},
	})
	if err != nil {
		t.Fatalf("Load(0): %v", err)
	}
	if cfg.SuppressionHistoryCount != 0 {
		t.Errorf("SuppressionHistoryCount = %d, want 0", cfg.SuppressionHistoryCount)
	}
}

func TestLoad_suppressionHistoryCountNegativeFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_SUPPRESSION_HISTORY_COUNT=-1"},
	})
	if err == nil {
		t.Fatal("Load: want error for negative STET_SUPPRESSION_HISTORY_COUNT, got nil")
	}
	if !strings.Contains(err.Error(), "STET_SUPPRESSION_HISTORY_COUNT") {
		t.Errorf("Load: want STET_SUPPRESSION_HISTORY_COUNT in error, got %v", err)
	}
}

func TestLoad_suppressionHistoryCountInvalidFromEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Load(ctx, LoadOptions{
		RepoRoot:         dir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{"STET_SUPPRESSION_HISTORY_COUNT=notanumber"},
	})
	if err == nil {
		t.Fatal("Load: want error for invalid STET_SUPPRESSION_HISTORY_COUNT, got nil")
	}
	if !strings.Contains(err.Error(), "STET_SUPPRESSION_HISTORY_COUNT") {
		t.Errorf("Load: want STET_SUPPRESSION_HISTORY_COUNT in error, got %v", err)
	}
}

func TestLoad_suppressionFromTOML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".review"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repoDir, ".review", "config.toml")
	if err := os.WriteFile(path, []byte("suppression_enabled = false\nsuppression_history_count = 10"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		RepoRoot:         repoDir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SuppressionEnabled {
		t.Errorf("SuppressionEnabled = true, want false (from TOML)")
	}
	if cfg.SuppressionHistoryCount != 10 {
		t.Errorf("SuppressionHistoryCount = %d, want 10", cfg.SuppressionHistoryCount)
	}
	// Explicit 0 means do not use history for suppression.
	if err := os.WriteFile(path, []byte("suppression_history_count = 0"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(ctx, LoadOptions{
		RepoRoot:         repoDir,
		GlobalConfigPath: filepath.Join(dir, "nope.toml"),
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SuppressionHistoryCount != 0 {
		t.Errorf("SuppressionHistoryCount = %d, want 0 (explicit TOML)", cfg.SuppressionHistoryCount)
	}
}

func TestLoad_optimizerScriptFromTOML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global.toml")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte(`optimizer_script = "python3 scripts/optimize.py"`), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cfg, err := Load(ctx, LoadOptions{
		GlobalConfigPath: globalPath,
		Env:              []string{},
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OptimizerScript != "python3 scripts/optimize.py" {
		t.Errorf("OptimizerScript = %q, want python3 scripts/optimize.py", cfg.OptimizerScript)
	}
}
