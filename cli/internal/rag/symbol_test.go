// Tests for the batched symbol lookup (symbol.go): batching must return the
// identical mapped definitions a sequential one-grep-per-symbol lookup would
// return, while spawning a single git grep process.
package rag

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"stet/cli/internal/git"
	"stet/cli/internal/testutil"
)

// TestMain isolates the package's tests from host git configuration and
// background git daemons (see cli/internal/testutil).
func TestMain(m *testing.M) {
	os.Exit(testutil.RunIsolatedGit(m))
}

// goDefPattern mirrors the Go resolvers' definition pattern shape.
func goDefPattern(symbol string) string {
	return `(func|type|var|const)[[:space:]]+` + regexp.QuoteMeta(symbol) + `([^a-zA-Z0-9_]|$)`
}

// initSymbolRepo creates a git repo with definitions spread over two files,
// including a symbol (Dup) with several matching lines and calls to a
// missing symbol.
func initSymbolRepo(t *testing.T) string {
	t.Helper()
	dir := testutil.TempGitDir(t)
	runCmd(t, dir, "init")
	runCmd(t, dir, "config", "user.email", "test@test")
	runCmd(t, dir, "config", "user.name", "Test")

	write(t, filepath.Join(dir, "alpha.go"), "package p\n\n// Alpha docs.\nfunc Alpha() int { return 1 }\n\n// Dup first.\nfunc Dup() {}\nfunc DupLater() {}\n")
	write(t, filepath.Join(dir, "beta.go"), "package p\n\n// Beta docs.\nfunc Beta() int { return Alpha() }\ntype Widget struct{}\nconst Pi = 3\n")
	runCmd(t, dir, "add", ".")
	runCmd(t, dir, "commit", "-m", "init")
	return dir
}

func runCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// sequentialLookup is the reference implementation: one git grep process per
// symbol, first matching hit wins. This is exactly the pre-batching behavior.
func sequentialLookup(ctx context.Context, repoRoot string, pairs []SymbolPattern) ([]*SymbolMatch, error) {
	results := make([]*SymbolMatch, len(pairs))
	for i, p := range pairs {
		hits, err := git.Grep(ctx, repoRoot, []string{p.Pattern})
		if err != nil {
			return nil, err
		}
		re := regexp.MustCompile(p.Pattern)
		for _, h := range hits {
			if !re.MatchString(h.Content) {
				continue
			}
			results[i] = &SymbolMatch{
				Symbol:  p.Symbol,
				RelPath: h.Path,
				AbsPath: filepath.Join(repoRoot, filepath.FromSlash(h.Path)),
				Line:    h.Line,
				Content: h.Content,
			}
			break
		}
	}
	return results, nil
}

// TestBatchGrepDefinitions_matchesSequentialLookup verifies that one batched
// grep produces the same per-symbol first hits as N sequential greps, with
// results aligned to the input pairs.
func TestBatchGrepDefinitions_matchesSequentialLookup(t *testing.T) {
	ctx := context.Background()
	repo := initSymbolRepo(t)

	pairs := []SymbolPattern{
		{Symbol: "Alpha", Pattern: goDefPattern("Alpha")},
		{Symbol: "Beta", Pattern: goDefPattern("Beta")},
		{Symbol: "Widget", Pattern: goDefPattern("Widget")},
		{Symbol: "Pi", Pattern: goDefPattern("Pi")},
		{Symbol: "Missing", Pattern: goDefPattern("Missing")}, // no definition
		{Symbol: "Dup", Pattern: goDefPattern("Dup")},         // matches two lines (Dup, DupLater? no: DupLater has trailing chars, excluded by boundary class)
	}
	wantPaths := map[string]string{
		"Alpha": "alpha.go", "Beta": "beta.go", "Widget": "beta.go",
		"Pi": "beta.go", "Dup": "alpha.go",
	}

	got, err := BatchGrepDefinitions(ctx, repo, pairs)
	if err != nil {
		t.Fatalf("BatchGrepDefinitions: %v", err)
	}
	want, err := sequentialLookup(ctx, repo, pairs)
	if err != nil {
		t.Fatalf("sequentialLookup: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range pairs {
		g, w := got[i], want[i]
		if (g == nil) != (w == nil) {
			t.Fatalf("pair %d (%s): got %+v, want %+v", i, pairs[i].Symbol, g, w)
		}
		if g == nil {
			continue
		}
		if g.Symbol != pairs[i].Symbol || w.Symbol != pairs[i].Symbol {
			t.Errorf("pair %d: symbol mapping got %q / want %q, input %q", i, g.Symbol, w.Symbol, pairs[i].Symbol)
		}
		if g.RelPath != w.RelPath || g.Line != w.Line || g.Content != w.Content || g.AbsPath != w.AbsPath {
			t.Errorf("pair %d (%s): got %+v, want %+v", i, pairs[i].Symbol, g, w)
		}
		if p, ok := wantPaths[pairs[i].Symbol]; ok && g.RelPath != p {
			t.Errorf("symbol %s resolved to %q, want %q", pairs[i].Symbol, g.RelPath, p)
		}
	}
}

func TestBatchGrepDefinitions_emptyPairs(t *testing.T) {
	got, err := BatchGrepDefinitions(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatalf("BatchGrepDefinitions: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for empty pairs; got %+v", got)
	}
}

// TestBatchGrepDefinitions_firstHitWins verifies the mapping rule: when a
// pattern matches multiple lines, the first hit in git's output order is
// mapped back to the symbol.
func TestBatchGrepDefinitions_firstHitWins(t *testing.T) {
	ctx := context.Background()
	repo := initSymbolRepo(t)

	got, err := BatchGrepDefinitions(ctx, repo, []SymbolPattern{
		{Symbol: "Dup", Pattern: goDefPattern("Dup")},
	})
	if err != nil {
		t.Fatalf("BatchGrepDefinitions: %v", err)
	}
	if len(got) != 1 || got[0] == nil {
		t.Fatalf("expected 1 match; got %+v", got)
	}
	// alpha.go: the first line matching the Dup pattern is line 7
	// ("func Dup() {}"); line 8 "func DupLater() {}" must not match
	// (identifier boundary) and must not displace the first hit.
	if got[0].Line != 7 {
		t.Errorf("Line = %d, want 7 (first matching definition)", got[0].Line)
	}
	if got[0].Content != "func Dup() {}" {
		t.Errorf("Content = %q, want %q", got[0].Content, "func Dup() {}")
	}
	if got[0].Symbol != "Dup" {
		t.Errorf("Symbol = %q, want Dup", got[0].Symbol)
	}
}

// TestBatchGrepDefinitions_resultOrderFollowsPairs verifies results stay
// aligned with the input pair order regardless of git's output order.
func TestBatchGrepDefinitions_resultOrderFollowsPairs(t *testing.T) {
	ctx := context.Background()
	repo := initSymbolRepo(t)

	// Beta is defined in beta.go, Alpha in alpha.go: git outputs alpha.go
	// first, but results must follow pair order (Beta first here).
	got, err := BatchGrepDefinitions(ctx, repo, []SymbolPattern{
		{Symbol: "Beta", Pattern: goDefPattern("Beta")},
		{Symbol: "Alpha", Pattern: goDefPattern("Alpha")},
		{Symbol: "Nope", Pattern: goDefPattern("Nope")},
	})
	if err != nil {
		t.Fatalf("BatchGrepDefinitions: %v", err)
	}
	if got[0] == nil || got[0].Symbol != "Beta" {
		t.Errorf("results[0] = %+v, want Beta match", got[0])
	}
	if got[1] == nil || got[1].Symbol != "Alpha" {
		t.Errorf("results[1] = %+v, want Alpha match", got[1])
	}
	if got[2] != nil {
		t.Errorf("results[2] = %+v, want nil for missing symbol", got[2])
	}
}
