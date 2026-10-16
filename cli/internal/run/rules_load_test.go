package run

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/diff"
	"stet/cli/internal/rules"
)

func writeRulesFixture(t *testing.T, repo string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(repo, ".cursor", "rules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadRulesForHunks(t *testing.T) {
	bigSections := strings.Repeat("### Check\n**Detect:** "+strings.Repeat("x", 400)+"\n\n", 20)
	tests := []struct {
		name        string
		budget      int
		warnDropped bool
		wantWarning bool
	}{
		{name: "fits_no_warning", budget: 100000, warnDropped: true, wantWarning: false},
		{name: "over_budget_warns_once", budget: 300, warnDropped: true, wantWarning: true},
		{name: "selector_path_never_warns", budget: 300, warnDropped: false, wantWarning: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := t.TempDir()
			stateDir := filepath.Join(repo, ".review")
			writeRulesFixture(t, repo, map[string]string{
				"py.mdc":     "---\nglobs: [\"**/*.py\"]\n---\n" + bigSections,
				"always.mdc": "---\nalwaysApply: true\n---\n" + bigSections,
			})
			hunks := []diff.Hunk{{FilePath: "a/app.py"}, {FilePath: "a/app.py"}, {FilePath: "b/lib.py"}}
			var out bytes.Buffer
			byFile := loadRulesForHunks(repo, stateDir, hunks, tt.budget, tt.warnDropped, &out)
			if len(byFile) != 2 || len(byFile["a/app.py"]) != 2 {
				t.Fatalf("byFile = %v, want 2 files with 2 rules", byFile)
			}
			if byFile["a/app.py"][0].Digest == "" {
				t.Error("loaded rules should carry a digest")
			}
			if _, err := os.Stat(filepath.Join(stateDir, rules.DigestCacheFileName)); err != nil {
				t.Errorf("digest cache not written: %v", err)
			}
			warned := strings.Contains(out.String(), "raise rules_max_tokens")
			if warned != tt.wantWarning {
				t.Errorf("warning = %q, want warning %v", out.String(), tt.wantWarning)
			}
			if strings.Count(out.String(), "\n") > 1 {
				t.Errorf("want at most one warning line, got %q", out.String())
			}
		})
	}
}

func TestLoadRulesForHunks_cacheSaveFailureWarns(t *testing.T) {
	repo := t.TempDir()
	writeRulesFixture(t, repo, map[string]string{"go.mdc": "---\nalwaysApply: true\n---\n### E\n**Detect:** x"})
	stateFile := filepath.Join(repo, "state-is-a-file")
	if err := os.WriteFile(stateFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	byFile := loadRulesForHunks(repo, stateFile, []diff.Hunk{{FilePath: "main.go"}}, 1000, true, &out)
	if len(byFile["main.go"]) != 1 {
		t.Fatalf("rules still load when the cache cannot be saved; got %v", byFile)
	}
	if !strings.Contains(out.String(), "rules digest cache not saved") {
		t.Errorf("want save failure warning, got %q", out.String())
	}
}
