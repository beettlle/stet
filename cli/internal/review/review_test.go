package review

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stet/cli/internal/diff"
	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
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

	list, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil)
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

	list, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil)
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
	_, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil)
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

	_, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil)
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

	list, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, nil)
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

	_, err := ReviewHunk(ctx, client, "m", dir, hunk, nil, userIntent)
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
