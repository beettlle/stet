package review

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/diff"
	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
	"stet/cli/internal/rules"
)

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

	list, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0)
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

	list, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0)
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

func TestReviewHunk_generateFails_returnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	client := ollama.NewClient(srv.URL, srv.Client())
	dir := t.TempDir()
	hunk := diff.Hunk{FilePath: "x.go", RawContent: "code", Context: "code"}
	ctx := context.Background()
	_, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0)
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

	_, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0)
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

	list, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, nil, "", 0)
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

	_, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, userIntent, nil, "", 0)
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

	_, err := ReviewHunk(ctx, client, "m", stateDir, hunk, nil, nil, nil, dir, 32768)
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

	_, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil, ruleList, "", 0)
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

