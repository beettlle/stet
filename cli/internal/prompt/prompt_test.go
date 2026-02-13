package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/diff"
	"stet/cli/internal/rag"
	"stet/cli/internal/rules"
)

func TestDefaultSystemPrompt_instructsActionability(t *testing.T) {
	got := DefaultSystemPrompt
	if !strings.Contains(got, "actionable") {
		t.Errorf("default prompt should instruct actionable issues; missing 'actionable'")
	}
	if !strings.Contains(got, "reverting intentional") {
		t.Errorf("default prompt should say do not suggest reverting intentional changes")
	}
	if !strings.Contains(got, "fewer") || !strings.Contains(got, "high-confidence") {
		t.Errorf("default prompt should prefer fewer, high-confidence findings")
	}
}

// TestDefaultSystemPrompt_instructsCoTAndAssumeValid asserts Phase 6.1 CoT prompt
// content: step-by-step verification, assume out-of-hunk identifiers valid, nitpick discard, User Intent.
// TestDefaultSystemPrompt_instructsDiffInterpretation asserts the prompt tells the model
// to review the resulting code and not report issues that the added lines fix (actionable findings).
func TestDefaultSystemPrompt_instructsDiffInterpretation(t *testing.T) {
	got := DefaultSystemPrompt
	if !strings.Contains(got, "resulting code") {
		t.Errorf("default prompt should instruct review of resulting code; missing 'resulting code'")
	}
	if !strings.Contains(got, "removed lines") || !strings.Contains(got, "added lines") {
		t.Errorf("default prompt should mention removed lines and added lines for diff interpretation")
	}
	if !strings.Contains(got, "Do not report issues that exist only in the removed lines") {
		t.Errorf("default prompt should say do not report issues only in removed lines that are fixed by added lines")
	}
}

func TestDefaultSystemPrompt_instructsCoTAndAssumeValid(t *testing.T) {
	got := DefaultSystemPrompt
	if !strings.Contains(got, "Step") && !strings.Contains(got, "steps") && !strings.Contains(strings.ToLower(got), "verify") {
		t.Errorf("default prompt should instruct step-by-step verification; missing 'Step'/'steps' or 'verify'")
	}
	if !strings.Contains(got, "assume") || !strings.Contains(got, "valid") {
		t.Errorf("default prompt should tell model to assume identifiers not in hunk are valid")
	}
	if !strings.Contains(got, "undefined") {
		t.Errorf("default prompt should mention not reporting undefined for out-of-hunk identifiers")
	}
	if !strings.Contains(got, "nitpick") || !strings.Contains(got, "discard") {
		t.Errorf("default prompt should instruct discard of nitpicks (self-correction)")
	}
	if !strings.Contains(got, "## User Intent") {
		t.Errorf("default prompt should include ## User Intent section (placeholder for 6.2)")
	}
}

func TestSystemPrompt_absentFile_returnsDefault(t *testing.T) {
	dir := t.TempDir()
	got, err := SystemPrompt(dir)
	if err != nil {
		t.Fatalf("SystemPrompt(%q): %v", dir, err)
	}
	if got != DefaultSystemPrompt {
		t.Errorf("expected default prompt; got length %d", len(got))
	}
	if !strings.Contains(got, "JSON array") {
		t.Errorf("default prompt should mention JSON array")
	}
	if !strings.Contains(got, "severity") || !strings.Contains(got, "category") {
		t.Errorf("default prompt should mention severity and category")
	}
	if !strings.Contains(got, "documentation") || !strings.Contains(got, "design") {
		t.Errorf("default prompt should list documentation and design categories")
	}
}

func TestSystemPrompt_presentFile_returnsFileContents(t *testing.T) {
	dir := t.TempDir()
	custom := "CUSTOM_PROMPT"
	path := filepath.Join(dir, optimizedPromptFilename)
	if err := os.WriteFile(path, []byte(custom), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	got, err := SystemPrompt(dir)
	if err != nil {
		t.Fatalf("SystemPrompt(%q): %v", dir, err)
	}
	if got != custom {
		t.Errorf("got %q, want %q", got, custom)
	}
}

func TestSystemPrompt_emptyStateDir_returnsDefault(t *testing.T) {
	got, err := SystemPrompt("")
	if err != nil {
		t.Fatalf("SystemPrompt(%q): %v", "", err)
	}
	if got != DefaultSystemPrompt {
		t.Errorf("expected default when stateDir empty")
	}
}

func TestSystemPrompt_fileWithWhitespace_trimmed(t *testing.T) {
	dir := t.TempDir()
	custom := "  TRIM_ME  \n"
	path := filepath.Join(dir, optimizedPromptFilename)
	if err := os.WriteFile(path, []byte(custom), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	got, err := SystemPrompt(dir)
	if err != nil {
		t.Fatalf("SystemPrompt(%q): %v", dir, err)
	}
	if got != "TRIM_ME" {
		t.Errorf("got %q, want TRIM_ME (TrimSpace)", got)
	}
}

func TestSystemPrompt_fileExistsButUnreadable_returnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, optimizedPromptFilename)
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatalf("create dir as optimized path: %v", err)
	}
	_, err := SystemPrompt(dir)
	if err == nil {
		t.Fatal("SystemPrompt should return error when path exists but is not a readable file")
	}
	if !strings.Contains(err.Error(), "read optimized prompt") {
		t.Errorf("error should mention read optimized prompt; got %q", err.Error())
	}
}

func TestUserPrompt_includesFilePathAndContent(t *testing.T) {
	hunk := diff.Hunk{
		FilePath:   "pkg/foo.go",
		RawContent: "@@ -1,3 +1,4 @@\n func bar() {\n+\treturn nil\n }",
		Context:    "@@ -1,3 +1,4 @@\n func bar() {\n+\treturn nil\n }",
	}
	got := UserPrompt(hunk)
	if !strings.Contains(got, "pkg/foo.go") {
		t.Errorf("UserPrompt should contain file path; got %q", got)
	}
	if !strings.Contains(got, "func bar()") {
		t.Errorf("UserPrompt should contain hunk content; got %q", got)
	}
	if !strings.HasPrefix(got, "File: pkg/foo.go") {
		t.Errorf("UserPrompt should start with File: path; got %q", got)
	}
}

func TestUserPrompt_emptyFilePath_returnsContentOnly(t *testing.T) {
	hunk := diff.Hunk{
		FilePath:   "",
		RawContent: "code",
		Context:    "code",
	}
	got := UserPrompt(hunk)
	if strings.Contains(got, "File:") {
		t.Errorf("empty file path should not add File: prefix; got %q", got)
	}
	if got != "code" {
		t.Errorf("got %q, want %q", got, "code")
	}
}

func TestUserPrompt_emptyContext_usesRawContent(t *testing.T) {
	hunk := diff.Hunk{
		FilePath:   "x.go",
		RawContent: "raw",
		Context:    "",
	}
	got := UserPrompt(hunk)
	if !strings.Contains(got, "raw") {
		t.Errorf("should use RawContent when Context empty; got %q", got)
	}
}

func TestInjectUserIntent_replacesPlaceholder(t *testing.T) {
	got := InjectUserIntent(DefaultSystemPrompt, "main", "fix")
	if !strings.Contains(got, "Branch: main") {
		t.Errorf("InjectUserIntent: want Branch: main; got:\n%s", got)
	}
	if !strings.Contains(got, "Commit: fix") {
		t.Errorf("InjectUserIntent: want Commit: fix; got:\n%s", got)
	}
	if strings.Contains(got, "(Not provided.)") {
		t.Errorf("InjectUserIntent: should not contain placeholder when intent provided")
	}
}

func TestInjectUserIntent_emptyUsesPlaceholder(t *testing.T) {
	got := InjectUserIntent(DefaultSystemPrompt, "", "")
	if !strings.Contains(got, "(Not provided.)") {
		t.Errorf("InjectUserIntent(empty): want (Not provided.); got:\n%s", got)
	}
}

func TestInjectUserIntent_optimizedPromptWithSection(t *testing.T) {
	custom := `Review code.

## User Intent
(custom placeholder)

## Review steps
Follow steps.`
	got := InjectUserIntent(custom, "feat/x", "Add feature")
	if !strings.Contains(got, "Branch: feat/x") {
		t.Errorf("InjectUserIntent: want Branch: feat/x; got:\n%s", got)
	}
	if !strings.Contains(got, "Commit: Add feature") {
		t.Errorf("InjectUserIntent: want Commit: Add feature; got:\n%s", got)
	}
	if strings.Contains(got, "(custom placeholder)") {
		t.Errorf("InjectUserIntent: should replace custom placeholder")
	}
	if !strings.Contains(got, "## Review steps") {
		t.Errorf("InjectUserIntent: should preserve following sections")
	}
}

func TestInjectUserIntent_missingSection_unchanged(t *testing.T) {
	noSection := "No User Intent section here at all."
	got := InjectUserIntent(noSection, "main", "fix")
	if got != noSection {
		t.Errorf("InjectUserIntent(missing section): want unchanged; got %q", got)
	}
}

func TestAppendCursorRules_nilRules_unchanged(t *testing.T) {
	base := "System prompt."
	got := AppendCursorRules(base, nil, "app.ts", 1000)
	if got != base {
		t.Errorf("AppendCursorRules(nil): want unchanged; got %q", got)
	}
}

func TestAppendCursorRules_emptyRules_unchanged(t *testing.T) {
	base := "System prompt."
	got := AppendCursorRules(base, []rules.CursorRule{}, "app.ts", 1000)
	if got != base {
		t.Errorf("AppendCursorRules(empty): want unchanged; got %q", got)
	}
}

func TestAppendCursorRules_noMatch_unchanged(t *testing.T) {
	base := "System prompt."
	ruleList := []rules.CursorRule{
		{Globs: []string{"*.go"}, Content: "Go rule"},
	}
	got := AppendCursorRules(base, ruleList, "app.ts", 1000)
	if got != base {
		t.Errorf("AppendCursorRules(no match): want unchanged; got %q", got)
	}
}

func TestAppendCursorRules_oneMatch_appendsSection(t *testing.T) {
	base := "System prompt."
	ruleList := []rules.CursorRule{
		{Globs: []string{"*.ts"}, Content: "Do not use console.log."},
	}
	got := AppendCursorRules(base, ruleList, "app.ts", 1000)
	if !strings.Contains(got, "## Project review criteria") {
		t.Errorf("AppendCursorRules: want section header; got:\n%s", got)
	}
	if !strings.Contains(got, "Do not use console.log.") {
		t.Errorf("AppendCursorRules: want rule body; got:\n%s", got)
	}
	if !strings.HasPrefix(got, base) {
		t.Errorf("AppendCursorRules: should start with original prompt")
	}
}

func TestAppendCursorRules_overTokenBudget_truncated(t *testing.T) {
	base := "System prompt."
	longBody := strings.Repeat("x", 5000)
	ruleList := []rules.CursorRule{
		{Globs: []string{"*"}, Content: longBody},
	}
	got := AppendCursorRules(base, ruleList, "any.go", 100)
	if !strings.Contains(got, "## Project review criteria") {
		t.Errorf("AppendCursorRules: want section header")
	}
	if !strings.Contains(got, "[truncated]") {
		t.Errorf("AppendCursorRules: want truncation marker when over budget")
	}
	if len(got) >= len(base)+len(longBody) {
		t.Errorf("AppendCursorRules: combined output should be shorter than full body")
	}
}

func TestAppendPromptShadows_empty_unchanged(t *testing.T) {
	base := "System prompt."
	got := AppendPromptShadows(base, nil)
	if got != base {
		t.Errorf("AppendPromptShadows(nil): want unchanged; got %q", got)
	}
	got = AppendPromptShadows(base, []Shadow{})
	if got != base {
		t.Errorf("AppendPromptShadows(empty): want unchanged; got %q", got)
	}
}

func TestAppendPromptShadows_nonEmpty_appendsSection(t *testing.T) {
	base := "System prompt."
	shadows := []Shadow{
		{FindingID: "f1", PromptContext: "code snippet one"},
	}
	got := AppendPromptShadows(base, shadows)
	if !strings.Contains(got, "## Negative examples (do not report)") {
		t.Errorf("AppendPromptShadows: want section header; got:\n%s", got)
	}
	if !strings.Contains(got, "code snippet one") {
		t.Errorf("AppendPromptShadows: want context in output; got:\n%s", got)
	}
	if !strings.Contains(got, "dismissed") {
		t.Errorf("AppendPromptShadows: want instruction about dismissed; got:\n%s", got)
	}
	if !strings.HasPrefix(got, base) {
		t.Errorf("AppendPromptShadows: should start with original prompt")
	}
}

func TestAppendPromptShadows_contextExceedsLimit_truncated(t *testing.T) {
	base := "System prompt."
	longCtx := strings.Repeat("x", 1000)
	shadows := []Shadow{
		{FindingID: "f1", PromptContext: longCtx},
	}
	got := AppendPromptShadows(base, shadows)
	if !strings.Contains(got, "## Negative examples (do not report)") {
		t.Errorf("AppendPromptShadows: want section header")
	}
	if !strings.Contains(got, "[truncated]") {
		t.Errorf("AppendPromptShadows: want truncation when context exceeds %d chars", maxShadowContextChars)
	}
	if len(got) >= len(base)+len(longCtx) {
		t.Errorf("AppendPromptShadows: should have truncated; output len %d >= base+full context %d", len(got), len(base)+len(longCtx))
	}
}

func TestAppendSymbolDefinitions_empty_unchanged(t *testing.T) {
	userPrompt := "File: foo.go\n\ncontent"
	got := AppendSymbolDefinitions(userPrompt, nil, 0)
	if got != userPrompt {
		t.Errorf("AppendSymbolDefinitions(nil): want unchanged prompt; got %q", got)
	}
	got = AppendSymbolDefinitions(userPrompt, []rag.Definition{}, 0)
	if got != userPrompt {
		t.Errorf("AppendSymbolDefinitions(empty): want unchanged prompt; got %q", got)
	}
}

func TestAppendSymbolDefinitions_injectsSectionAndSignature(t *testing.T) {
	userPrompt := "File: pkg/foo.go\n\n+hunk line"
	defs := []rag.Definition{
		{Symbol: "Bar", File: "pkg/foo.go", Line: 5, Signature: "func Bar() int", Docstring: "Bar does something."},
	}
	got := AppendSymbolDefinitions(userPrompt, defs, 0)
	if !strings.Contains(got, symbolDefinitionsHeader) {
		t.Errorf("AppendSymbolDefinitions: want section header; got:\n%s", got)
	}
	if !strings.Contains(got, "func Bar() int") {
		t.Errorf("AppendSymbolDefinitions: want signature; got:\n%s", got)
	}
	if !strings.Contains(got, "Bar does something.") {
		t.Errorf("AppendSymbolDefinitions: want docstring; got:\n%s", got)
	}
	if !strings.Contains(got, "pkg/foo.go") || !strings.Contains(got, "Line: 5") {
		t.Errorf("AppendSymbolDefinitions: want file and line; got:\n%s", got)
	}
	if !strings.HasPrefix(got, userPrompt) {
		t.Errorf("AppendSymbolDefinitions: should start with original user prompt")
	}
}
