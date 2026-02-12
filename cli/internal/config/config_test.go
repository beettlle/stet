package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func ptrStr(s string) *string { return &s }

func TestDefaultConfig(t *testing.T) {
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
	if c.StateDir != "" || c.WorktreeRoot != "" {
		t.Errorf("StateDir or WorktreeRoot non-empty: %q, %q", c.StateDir, c.WorktreeRoot)
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
		cfg.Timeout != want.Timeout {
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
