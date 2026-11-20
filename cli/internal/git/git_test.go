// Tests for the batched git grep primitive (git.go).
package git

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// TestGrep_singleProcessForAllPatterns verifies that one Grep call with many
// patterns spawns exactly one git process and passes every pattern as a
// separate -e argument. This is the batching property SP-087 requires: N
// symbol lookups must cost 1 process, not N.
func TestGrep_singleProcessForAllPatterns(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "code.go", "package p\n\nfunc Alpha() {}\nfunc Beta() {}\nfunc Gamma() {}\n")
	run(t, repo, "git", "add", "code.go")
	run(t, repo, "git", "commit", "-m", "code")

	calls := 0
	var capturedArgs []string
	orig := grepCommand
	grepCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls++
		capturedArgs = append([]string(nil), args...)
		return orig(ctx, name, args...)
	}
	defer func() { grepCommand = orig }()

	patterns := []string{`func Alpha`, `func Beta`, `func Gamma`}
	hits, err := Grep(context.Background(), repo, patterns)
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 git process for %d patterns; got %d", len(patterns), calls)
	}
	if got, want := capturedArgs[0], "grep"; got != want {
		t.Errorf("args[0] = %q, want %q", got, want)
	}
	for i, p := range patterns {
		if got := capturedArgs[3+2*i]; got != "-e" {
			t.Errorf("args[%d] = %q, want -e", 3+2*i, got)
		}
		if got := capturedArgs[4+2*i]; got != p {
			t.Errorf("pattern arg = %q, want %q", got, p)
		}
	}
	if len(hits) != 3 {
		t.Fatalf("expected 3 hits; got %d: %+v", len(hits), hits)
	}
	want := map[string]int{"func Alpha() {}": 3, "func Beta() {}": 4, "func Gamma() {}": 5}
	for _, h := range hits {
		if h.Path != "code.go" {
			t.Errorf("Path = %q, want code.go", h.Path)
		}
		if want[h.Content] != h.Line {
			t.Errorf("hit content %q line = %d, want %d", h.Content, h.Line, want[h.Content])
		}
	}
}

func TestGrep_noMatch_returnsNilNoError(t *testing.T) {
	repo := initRepo(t)
	hits, err := Grep(context.Background(), repo, []string{`noSuchSymbolHere`})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if hits != nil {
		t.Errorf("expected nil hits for no match; got %+v", hits)
	}
}

func TestGrep_emptyPatterns_skipsProcess(t *testing.T) {
	repo := initRepo(t)
	calls := 0
	orig := grepCommand
	grepCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls++
		return orig(ctx, name, args...)
	}
	defer func() { grepCommand = orig }()

	hits, err := Grep(context.Background(), repo, nil)
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if hits != nil {
		t.Errorf("expected nil hits for no patterns; got %+v", hits)
	}
	if calls != 0 {
		t.Errorf("expected no process for empty pattern list; got %d calls", calls)
	}
}

func TestParseGrepOutput(t *testing.T) {
	out := strings.Join([]string{
		"pkg/foo.go:10:\tTarget()",
		"pkg/foo.go:12:\tOther()",
		"malformed-line",
		":5:empty path",
		"../escape.go:1:nope",
		"a.txt:0:zero lineno",
		"a.txt:notanum:bad lineno",
		"", // trailing newline
	}, "\n")
	hits := parseGrepOutput(out)
	if len(hits) != 2 {
		t.Fatalf("expected 2 parsed hits; got %d: %+v", len(hits), hits)
	}
	if hits[0].Path != "pkg/foo.go" || hits[0].Line != 10 || hits[0].Content != "\tTarget()" {
		t.Errorf("hit[0] = %+v", hits[0])
	}
	if hits[1].Path != "pkg/foo.go" || hits[1].Line != 12 || hits[1].Content != "\tOther()" {
		t.Errorf("hit[1] = %+v", hits[1])
	}
}

func TestParseGrepOutput_empty(t *testing.T) {
	if hits := parseGrepOutput("  \n"); hits != nil {
		t.Errorf("expected nil for blank output; got %+v", hits)
	}
}
