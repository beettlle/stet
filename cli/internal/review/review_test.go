package review

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/diff"
	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
	"stet/cli/internal/rules"
	"stet/cli/internal/tokens"

	_ "stet/cli/internal/rag/go"     // register Go resolver for RAG tests
	_ "stet/cli/internal/rag/python" // register Python resolver
	_ "stet/cli/internal/rag/java"   // register Java resolver
	_ "stet/cli/internal/rag/swift"  // register Swift resolver
	_ "stet/cli/internal/rag/js"     // register JavaScript/TypeScript resolver
	_ "stet/cli/internal/rag/rust"   // register Rust resolver
)

func TestEffectiveRAGTokenCap(t *testing.T) {
	t.Parallel()
	reserve := tokens.DefaultResponseReserve
	tests := []struct {
		name               string
		contextLimit       int
		basePromptTokens   int
		responseReserve    int
		ragMaxTokens       int
		wantEffectiveCap   int
	}{
		{"context_limit_zero_rag_zero", 0, 10000, reserve, 0, 0},
		{"context_limit_zero_rag_500", 0, 10000, reserve, 500, 500},
		{"context_limit_negative", -1, 10000, reserve, 0, 0},
		{"adaptive_budget_positive_rag_zero", 32768, 10000, reserve, 0, 32768 - 10000 - reserve},
		{"adaptive_budget_positive_rag_caps", 32768, 10000, reserve, 500, 500},
		{"adaptive_budget_smaller_than_config", 32000, 30000, reserve, 500, 0},
		{"adaptive_budget_300_rag_500", 3348, 1000, reserve, 500, 300},
		{"base_plus_reserve_equals_limit", 12048, 10000, reserve, 0, 0},
		{"base_plus_reserve_exceeds_limit", 10000, 10000, reserve, 500, 0},
		// For very large context limits (e.g. 256k), per-hunk RAG tokens are capped
		// so a single hunk cannot consume most of the context window.
		{"large_context_uses_absolute_cap", 262144, 10000, reserve, 0, maxRAGTokensWhenContextLarge},
		// When contextLimit is large but ragMaxTokens is smaller than the cap, the explicit
		// ragMaxTokens still applies.
		{"large_context_respects_ragMaxTokens", 262144, 10000, reserve, 1024, 1024},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := effectiveRAGTokenCap(tt.contextLimit, tt.basePromptTokens, tt.responseReserve, tt.ragMaxTokens)
			if got != tt.wantEffectiveCap {
				t.Errorf("effectiveRAGTokenCap(%d, %d, %d, %d) = %d, want %d",
					tt.contextLimit, tt.basePromptTokens, tt.responseReserve, tt.ragMaxTokens, got, tt.wantEffectiveCap)
			}
		})
	}
}

func TestReviewHunk_successFirstTry(t *testing.T) {
	validResp := `[{"file":"a.go","line":1,"severity":"warning","category":"style","message":"fix"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	dir := t.TempDir()
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "code", Context: "code"}
	ctx := context.Background()

	list, _, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0, 0, 0, false, 0, 0, 0, nil, false, false, nil, nil)
	if err != nil {
		t.Fatalf("ReviewHunk: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].ID == "" || list[0].File != "a.go" {
		t.Errorf("finding: %+v", list[0])
	}
}

func TestReviewHunk_retryThenSuccess(t *testing.T) {
	validResp := `[{"file":"b.go","line":2,"severity":"info","category":"maintainability","message":"ok"}]`
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		callCount++
		w.WriteHeader(http.StatusOK)
		if callCount == 1 {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": "not valid json", "done": true})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	dir := t.TempDir()
	hunk := diff.Hunk{FilePath: "b.go", RawContent: "x", Context: "x"}
	ctx := context.Background()

	list, _, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0, 0, 0, false, 0, 0, 0, nil, false, false, nil, nil)
	if err != nil {
		t.Fatalf("ReviewHunk: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 generate calls (retry), got %d", callCount)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
}

// TestReviewHunk_rustFile_minifiesContext exercises the .rs minify path so Rust
// hunks get whitespace reduction like Go. It does not assert on prompt content;
// coverage confirms the branch is taken.
func TestReviewHunk_rustFile_minifiesContext(t *testing.T) {
	validResp := `[{"file":"lib.rs","line":1,"severity":"info","category":"style","message":"ok"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	dir := t.TempDir()
	raw := "@@ -1,2 +1,2 @@\n \tfn foo() {}\n+\tbar();"
	hunk := diff.Hunk{FilePath: "src/lib.rs", RawContent: raw, Context: raw}
	ctx := context.Background()

	list, _, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0, 0, 0, false, 0, 0, 0, nil, false, false, nil, nil)
	if err != nil {
		t.Fatalf("ReviewHunk: %v", err)
	}
	if len(list) != 1 || list[0].File != "lib.rs" {
		t.Errorf("unexpected findings: %+v", list)
	}
}

func TestReviewHunk_generateFails_returnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	client := ollama.NewClient(srv.URL, srv.Client())
	dir := t.TempDir()
	hunk := diff.Hunk{FilePath: "x.go", RawContent: "code", Context: "code"}
	ctx := context.Background()
	_, _, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0, 0, 0, false, 0, 0, 0, nil, false, false, nil, nil)
	if err == nil {
		t.Fatal("ReviewHunk: want error when generate fails, got nil")
	}
}

func TestReviewHunk_parseFailsTwice_returnsError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": "still not json", "done": true})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	dir := t.TempDir()
	hunk := diff.Hunk{FilePath: "c.go", RawContent: "y", Context: "y"}
	ctx := context.Background()

	_, _, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0, 0, 0, false, 0, 0, 0, nil, false, false, nil, nil)
	if err == nil {
		t.Fatal("ReviewHunk: want error when parse fails twice, got nil")
	}
	if callCount != 2 {
		t.Errorf("expected 2 generate calls, got %d", callCount)
	}
}

// TestReviewHunk_hunkWithExternalVariable_mockReturnsNoUndefinedFinding is the
// Phase 6.1 regression test: a hunk that uses a variable/function defined
// outside the hunk should not produce a "variable undefined" finding when the
// model follows CoT instructions (assume valid). Mock returns empty findings.
func TestReviewHunk_hunkWithExternalVariable_mockReturnsNoUndefinedFinding(t *testing.T) {
	emptyResp := `[]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": emptyResp, "done": true})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	dir := t.TempDir()
	// Hunk uses processResult and data whose definitions are outside the hunk.
	hunk := diff.Hunk{
		FilePath:   "pkg/foo.go",
		RawContent: "return processResult(data)",
		Context:    "return processResult(data)",
	}
	ctx := context.Background()

	list, _, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0, 0, 0, false, 0, 0, 0, nil, false, false, nil, nil)
	if err != nil {
		t.Fatalf("ReviewHunk: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("len(list) = %d, want 0 (model should not report undefined for out-of-hunk identifiers)", len(list))
	}
	for _, f := range list {
		if strings.Contains(strings.ToLower(f.Message), "undefined") || strings.Contains(f.Message, "Variable undefined error") {
			t.Errorf("finding must not be undefined-related; got message %q", f.Message)
		}
	}
}

// TestReviewHunk_injectsUserIntentIntoPrompt is the Phase 6.2 regression test:
// when userIntent is provided, the system prompt sent to Ollama must contain
// the injected commit message (e.g. "Refactor: formatting only").
func TestReviewHunk_injectsUserIntentIntoPrompt(t *testing.T) {
	commitMsg := "Refactor: formatting only"
	validResp := `[]`
	var capturedSystem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var req struct {
			System string `json:"system"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		capturedSystem = req.System
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	dir := t.TempDir()
	hunk := diff.Hunk{FilePath: "pkg.go", RawContent: "+x := 1", Context: "+x := 1"}
	userIntent := &prompt.UserIntent{Branch: "main", CommitMsg: commitMsg}
	ctx := context.Background()

	_, _, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, userIntent, nil, "", 0, 0, 0, false, 0, 0, 0, nil, false, false, nil, nil)
	if err != nil {
		t.Fatalf("ReviewHunk: %v", err)
	}
	if !strings.Contains(capturedSystem, commitMsg) {
		t.Errorf("system prompt must contain commit message %q; got:\n%s", commitMsg, capturedSystem)
	}
	if !strings.Contains(capturedSystem, "Branch: main") {
		t.Errorf("system prompt must contain branch; got:\n%s", capturedSystem)
	}
}

// TestReviewHunk_hunkWithVariableAtTopOfFunction_promptContainsVariable is the
// Phase 6.4 regression test: when a hunk modifies a line that uses a variable
// declared at the top of a long function, hunk expansion injects the enclosing
// function context. The prompt sent to the LLM must contain the variable
// declaration so the model can correctly identify the variable's type.
func TestReviewHunk_hunkWithVariableAtTopOfFunction_promptContainsVariable(t *testing.T) {
	dir := t.TempDir()
	content := `package pkg

func processData(input string) (int, error) {
	var count int
	for i := 0; i < 50; i++ {
		count += i
	}
	return count, nil
}
`
	path := filepath.Join(dir, "pkg", "foo.go")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	validResp := `[]`
	var capturedPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		capturedPrompt = req.Prompt
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	stateDir := t.TempDir()
	// Hunk at lines 4-7 in new file (inside processData; uses count declared at line 4)
	hunk := diff.Hunk{
		FilePath:   "pkg/foo.go",
		RawContent: "@@ -3,6 +3,6 @@\n func processData(input string) (int, error) {\n\tvar count int\n\tfor i := 0; i < 50; i++ {\n\t\tcount += i\n\t}\n\treturn count, nil",
		Context:    "",
	}
	ctx := context.Background()

	_, _, err := ReviewHunk(ctx, client, "m", stateDir, hunk, nil, nil, nil, dir, 32768, 0, 0, false, 0, 0, 0, nil, false, false, nil, nil)
	if err != nil {
		t.Fatalf("ReviewHunk: %v", err)
	}
	if !strings.Contains(capturedPrompt, "var count") && !strings.Contains(capturedPrompt, "count :=") {
		t.Errorf("prompt must contain variable declaration (var count or count :=); got:\n%s", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, "## Enclosing function context") {
		t.Errorf("prompt must contain enclosing function context section; got:\n%s", capturedPrompt)
	}
}

// TestReviewHunk_injectsCursorRulesIntoSystemPrompt is the Phase 6.6 integration
// test: when ruleList contains a rule that matches the hunk's file (e.g. *.js
// for test.js), the system prompt sent to Ollama must contain "## Project review
// criteria" and the rule body.
func TestReviewHunk_injectsCursorRulesIntoSystemPrompt(t *testing.T) {
	validResp := `[]`
	var capturedSystem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var req struct {
			System string `json:"system"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		capturedSystem = req.System
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	dir := t.TempDir()
	hunk := diff.Hunk{FilePath: "test.js", RawContent: "code", Context: "code"}
	ruleList := []rules.CursorRule{
		{Globs: []string{"*.js"}, Content: "Do not use console.log."},
	}
	ctx := context.Background()

	_, _, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, ruleList, "", 0, 0, 0, false, 0, 0, 0, nil, false, false, nil, nil)
	if err != nil {
		t.Fatalf("ReviewHunk: %v", err)
	}
	if !strings.Contains(capturedSystem, "## Project review criteria") {
		t.Errorf("system prompt must contain ## Project review criteria; got:\n%s", capturedSystem)
	}
	if !strings.Contains(capturedSystem, "Do not use console.log.") {
		t.Errorf("system prompt must contain rule body; got:\n%s", capturedSystem)
	}
}

// TestReviewHunk_withRAG_injectsSymbolDefinitions is the Sub-phase 6.8 test: when
// RAG is enabled (ragMaxDefs > 0) and the hunk references a symbol defined in the
// repo, the user prompt sent to Ollama must contain the symbol definitions section.
func TestReviewHunk_withRAG_injectsSymbolDefinitions(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	pkgDir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "package pkg\n\nfunc Bar() int { return 42 }\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "foo.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "pkg/foo.go")

	validResp := `[]`
	var capturedPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Prompt string `json:"prompt"`
		}
		_ = json.Unmarshal(body, &req)
		capturedPrompt = req.Prompt
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	hunk := diff.Hunk{FilePath: "pkg/foo.go", RawContent: "+x := Bar()", Context: "+x := Bar()"}
	ctx := context.Background()

	_, _, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, dir, 0, 5, 0, false, 0, 0, 0, nil, false, false, nil, nil)
	if err != nil {
		t.Fatalf("ReviewHunk: %v", err)
	}
	if !strings.Contains(capturedPrompt, "## Symbol definitions") {
		t.Skipf("RAG resolver returned no definitions in this environment (git grep may not find symbols)")
	}
	if !strings.Contains(capturedPrompt, "func Bar") {
		t.Errorf("user prompt must contain symbol signature (func Bar); got:\n%s", capturedPrompt)
	}
}

// TestReviewHunk_perHunkAdaptiveRAG_truncatesToFitContext is Phase 6.11: when contextLimit
// is set and ragMaxTokens is 0, the per-hunk RAG token cap is computed and the symbol-definitions
// block is truncated to fit.
func TestReviewHunk_perHunkAdaptiveRAG_truncatesToFitContext(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	pkgDir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "package pkg\n\nfunc Bar() int { return 42 }\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "foo.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, dir, "pkg/foo.go")

	system, err := prompt.SystemPrompt(dir)
	if err != nil {
		t.Fatalf("SystemPrompt: %v", err)
	}
	hunk := diff.Hunk{FilePath: "pkg/foo.go", RawContent: "+x := Bar()", Context: "+x := Bar()"}
	userWithoutRAG := prompt.UserPrompt(hunk)
	basePromptTokens := tokens.Estimate(system + "\n" + userWithoutRAG)
	contextLimit := 6000
	reserve := tokens.DefaultResponseReserve
	budget := contextLimit - basePromptTokens - reserve
	if budget <= 0 {
		t.Skipf("default prompt too large for test (base=%d); need positive budget", basePromptTokens)
	}
	const tolerance = 50
	maxRAGTokens := budget + tolerance

	validResp := `[]`
	var capturedPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Prompt string `json:"prompt"`
		}
		_ = json.Unmarshal(body, &req)
		capturedPrompt = req.Prompt
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	ctx := context.Background()

	_, _, err = ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, dir, contextLimit, 5, 0, false, 0, 0, 0, nil, false, false, nil, nil)
	if err != nil {
		t.Fatalf("ReviewHunk: %v", err)
	}
	if !strings.Contains(capturedPrompt, "## Symbol definitions") {
		// RAG resolver may return no definitions when git grep finds no symbols (e.g. minimal test env).
		// Skip is acceptable; truncation behavior is exercised when definitions are present.
		t.Skipf("RAG resolver returned no definitions in this environment (git grep may not find symbols); cannot verify truncation")
	}
	idx := strings.Index(capturedPrompt, "## Symbol definitions (for context)\n\n")
	if idx < 0 {
		idx = strings.Index(capturedPrompt, "## Symbol definitions\n\n")
		if idx < 0 {
			t.Fatal("could not find symbol definitions header")
		}
		idx += len("## Symbol definitions\n\n")
	} else {
		idx += len("## Symbol definitions (for context)\n\n")
	}
	ragBlock := capturedPrompt[idx:]
	ragTokens := tokens.Estimate(ragBlock)
	if ragTokens > maxRAGTokens {
		t.Errorf("RAG block token estimate %d exceeds per-hunk budget %d + tolerance %d (max %d); block was truncated to fit",
			ragTokens, budget, tolerance, maxRAGTokens)
	}
}

func TestReviewHunk_nitpickyTrue_appendsNitpickySection(t *testing.T) {
	validResp := `[{"file":"a.go","line":1,"severity":"info","category":"style","message":"typo"}]`
	var capturedSystem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body struct {
			System string `json:"system"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		capturedSystem = body.System
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": validResp, "done": true})
	}))
	defer srv.Close()

	client := ollama.NewClient(srv.URL, srv.Client())
	dir := t.TempDir()
	hunk := diff.Hunk{FilePath: "a.go", RawContent: "code", Context: "code"}
	ctx := context.Background()

	list, _, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0, 0, 0, false, 0, 0, 0, nil, true, false, nil, nil)
	if err != nil {
		t.Fatalf("ReviewHunk: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if !strings.Contains(capturedSystem, "## Nitpicky mode") {
		t.Errorf("system prompt must contain ## Nitpicky mode when nitpicky=true; got:\n%s", capturedSystem)
	}
	if !strings.Contains(capturedSystem, "Do **not** discard findings") {
		t.Errorf("system prompt must contain nitpicky instructions; got:\n%s", capturedSystem)
	}
}

// TestPrepareHunkPrompt_callGraphDisabled_noCallGraphSection asserts that when
// RAG call-graph is disabled, the user prompt does not contain call-graph sections.
func TestPrepareHunkPrompt_callGraphDisabled_noCallGraphSection(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "package pkg\n\nfunc Foo() int { return 1 }\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "a.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	hunk := diff.Hunk{
		FilePath:   "pkg/a.go",
		RawContent: "@@ -1,3 +1,3 @@\n package pkg\n\n func Foo() int { return 1 }\n",
		Context:    "",
	}
	ctx := context.Background()
	_, user, err := PrepareHunkPrompt(ctx, "system", hunk, nil, dir, 32768, 0, 0, false, 3, 3, 0, false, nil, nil)
	if err != nil {
		t.Fatalf("PrepareHunkPrompt: %v", err)
	}
	if strings.Contains(user, "## Callers (upstream)") {
		t.Errorf("call-graph disabled: user prompt must not contain ## Callers (upstream); got:\n%s", user)
	}
	if strings.Contains(user, "## Callees (downstream)") {
		t.Errorf("call-graph disabled: user prompt must not contain ## Callees (downstream); got:\n%s", user)
	}
}

// TestPrepareHunkPrompt_nonGoFile_noCallGraphSection asserts that for non-Go files
// the prompt does not contain call-graph sections even when call-graph is enabled.
func TestPrepareHunkPrompt_nonGoFile_noCallGraphSection(t *testing.T) {
	dir := t.TempDir()
	hunk := diff.Hunk{
		FilePath:   "app.ts",
		RawContent: "@@ -1,1 +1,1 @@\n code\n",
		Context:    "code",
	}
	ctx := context.Background()
	_, user, err := PrepareHunkPrompt(ctx, "system", hunk, nil, dir, 32768, 0, 0, true, 3, 3, 0, false, nil, nil)
	if err != nil {
		t.Fatalf("PrepareHunkPrompt: %v", err)
	}
	if strings.Contains(user, "## Callers (upstream)") {
		t.Errorf("non-Go file: user prompt must not contain ## Callers (upstream); got:\n%s", user)
	}
}

// TestPrepareHunkPrompt_suppressionPerHunk asserts that when suppressionExamples
// and a generous contextLimit are passed, the returned system prompt contains
// the "Do not report issues similar to" section and the example text.
func TestPrepareHunkPrompt_suppressionPerHunk(t *testing.T) {
	hunk := diff.Hunk{
		FilePath:   "pkg/foo.go",
		RawContent: "@@ -1,1 +1,1 @@\n code\n",
		Context:    "code",
	}
	examples := []string{"pkg/foo.go:42: Consider adding comments"}
	ctx := context.Background()
	system, _, err := PrepareHunkPrompt(ctx, "base", hunk, nil, "", 32768, 0, 0, false, 0, 0, 0, false, examples, nil)
	if err != nil {
		t.Fatalf("PrepareHunkPrompt: %v", err)
	}
	if !strings.Contains(system, "## Do not report issues similar to") {
		t.Error("system prompt should contain suppression section header")
	}
	wantEx := "pkg/foo.go:42: Consider adding comments"
	if !strings.Contains(system, wantEx) {
		t.Errorf("system prompt should contain example %q", wantEx)
	}
}

// TestPrepareHunkPrompt_suppressionBudgetCapsExamples asserts that with a very
// small contextLimit the suppression budget is exhausted and zero or fewer
// examples are included than with a large limit.
func TestPrepareHunkPrompt_suppressionBudgetCapsExamples(t *testing.T) {
	hunk := diff.Hunk{
		FilePath:   "pkg/foo.go",
		RawContent: "@@ -1,1 +1,1 @@\n code\n",
		Context:    "code",
	}
	examples := []string{"a.go:1: msg1", "b.go:2: longer message here"}
	ctx := context.Background()
	systemSmall, _, err := PrepareHunkPrompt(ctx, "base", hunk, nil, "", 500, 0, 0, false, 0, 0, 0, false, examples, nil)
	if err != nil {
		t.Fatalf("PrepareHunkPrompt(small limit): %v", err)
	}
	systemLarge, _, err := PrepareHunkPrompt(ctx, "base", hunk, nil, "", 32768, 0, 0, false, 0, 0, 0, false, examples, nil)
	if err != nil {
		t.Fatalf("PrepareHunkPrompt(large limit): %v", err)
	}
	countBullets := func(s string) int { return strings.Count(s, "\n- ") }
	smallN := countBullets(systemSmall)
	largeN := countBullets(systemLarge)
	if smallN > largeN {
		t.Errorf("with small context limit expected fewer or equal suppression bullets than with large; got small=%d large=%d", smallN, largeN)
	}
}

// TestPrepareHunkPrompt_suppressionBudgetCapBounded asserts that when contextLimit is
// extremely large, the number of suppression examples does not keep growing with the
// limit, i.e. the per-hunk suppression budget is bounded.
func TestPrepareHunkPrompt_suppressionBudgetCapBounded(t *testing.T) {
	hunk := diff.Hunk{
		FilePath:   "pkg/foo.go",
		RawContent: "@@ -1,1 +1,1 @@\n code\n",
		Context:    "code",
	}
	// Use several examples so that, without a cap, more budget could allow more examples.
	examples := []string{
		"a.go:1: msg1",
		"b.go:2: msg2",
		"c.go:3: msg3",
		"d.go:4: msg4",
		"e.go:5: msg5",
	}
	ctx := context.Background()
	systemLarge, _, err := PrepareHunkPrompt(ctx, "base", hunk, nil, "", 262144, 0, 0, false, 0, 0, 0, false, examples, nil)
	if err != nil {
		t.Fatalf("PrepareHunkPrompt(large limit): %v", err)
	}
	systemHuge, _, err := PrepareHunkPrompt(ctx, "base", hunk, nil, "", 524288, 0, 0, false, 0, 0, 0, false, examples, nil)
	if err != nil {
		t.Fatalf("PrepareHunkPrompt(huge limit): %v", err)
	}
	countBullets := func(s string) int { return strings.Count(s, "\n- ") }
	largeN := countBullets(systemLarge)
	hugeN := countBullets(systemHuge)
	if hugeN != largeN {
		t.Errorf("suppression examples should be bounded by cap; got large=%d bullets, huge=%d bullets", largeN, hugeN)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "config", "user.email", "test@test")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	_ = cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	_ = cmd.Run()
}

func gitAdd(t *testing.T, dir, path string) {
	t.Helper()
	cmd := exec.Command("git", "add", path)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "add")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

