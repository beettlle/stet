package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/diff"
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
