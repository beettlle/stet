package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stet/cli/internal/diff"
)

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
