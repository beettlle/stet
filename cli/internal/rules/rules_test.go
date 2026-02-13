package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMDC_globsString_contentParsed(t *testing.T) {
	content := "---\nglobs: \"*.ts\"\n---\nDo not use any.\n"
	rule, ok := parseMDC(content)
	if !ok {
		t.Fatal("parseMDC: want true")
	}
	if len(rule.Globs) != 1 || rule.Globs[0] != "*.ts" {
		t.Errorf("Globs = %v, want [*.ts]", rule.Globs)
	}
	if rule.AlwaysApply {
		t.Error("AlwaysApply = true, want false")
	}
	if rule.Content != "Do not use any." {
		t.Errorf("Content = %q, want %q", rule.Content, "Do not use any.")
	}
}

func TestParseMDC_alwaysApplyAndListGlobs(t *testing.T) {
	content := "---\nalwaysApply: true\nglobs:\n  - foo/*\n  - bar/*\n---\nApply everywhere.\n"
	rule, ok := parseMDC(content)
	if !ok {
		t.Fatal("parseMDC: want true")
	}
	if !rule.AlwaysApply {
		t.Error("AlwaysApply = false, want true")
	}
	if len(rule.Globs) != 2 {
		t.Errorf("Globs len = %d, want 2", len(rule.Globs))
	}
	if rule.Content != "Apply everywhere." {
		t.Errorf("Content = %q", rule.Content)
	}
}

func TestParseMDC_invalidYAML_returnsFalse(t *testing.T) {
	content := "---\ninvalid: [\n---\nbody"
	_, ok := parseMDC(content)
	if ok {
		t.Error("parseMDC: want false for invalid YAML")
	}
}

func TestParseMDC_noFrontmatter_returnsFalse(t *testing.T) {
	content := "no delimiters"
	_, ok := parseMDC(content)
	if ok {
		t.Error("parseMDC: want false when no ---")
	}
}

func TestFilterRules_globMatchTS(t *testing.T) {
	rules := []CursorRule{
		{Globs: []string{"*.ts"}, Content: "TypeScript rule"},
	}
	matched := FilterRules(rules, "app.ts")
	if len(matched) != 1 {
		t.Fatalf("len(matched) = %d, want 1", len(matched))
	}
	if matched[0].Content != "TypeScript rule" {
		t.Errorf("Content = %q", matched[0].Content)
	}
}

func TestFilterRules_globNoMatchGo(t *testing.T) {
	rules := []CursorRule{
		{Globs: []string{"*.ts"}, Content: "TypeScript rule"},
	}
	matched := FilterRules(rules, "main.go")
	if len(matched) != 0 {
		t.Errorf("len(matched) = %d, want 0", len(matched))
	}
}

func TestFilterRules_pathGlob_match(t *testing.T) {
	rules := []CursorRule{
		{Globs: []string{"foo/*"}, Content: "Foo rule"},
	}
	matched := FilterRules(rules, "foo/x.js")
	if len(matched) != 1 {
		t.Fatalf("len(matched) = %d, want 1", len(matched))
	}
	matched = FilterRules(rules, "bar/x.js")
	if len(matched) != 0 {
		t.Errorf("len(matched) for bar/x.js = %d, want 0", len(matched))
	}
}

func TestFilterRules_alwaysApply_included(t *testing.T) {
	rules := []CursorRule{
		{Globs: []string{"*.ts"}, Content: "TS"},
		{AlwaysApply: true, Content: "Global"},
	}
	matched := FilterRules(rules, "main.go")
	if len(matched) != 1 {
		t.Fatalf("len(matched) = %d, want 1 (alwaysApply only)", len(matched))
	}
	if matched[0].Content != "Global" {
		t.Errorf("Content = %q", matched[0].Content)
	}
}

func TestFilterRules_emptyRules_returnsNil(t *testing.T) {
	matched := FilterRules(nil, "x.go")
	if matched != nil {
		t.Errorf("FilterRules(nil) = %v, want nil", matched)
	}
	matched = FilterRules([]CursorRule{}, "x.go")
	if matched != nil {
		t.Errorf("FilterRules([]) = %v, want nil", matched)
	}
}

func TestLoadRules_nonExistentDir_returnsNilNil(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	rules, err := LoadRules(dir)
	if err != nil {
		t.Errorf("LoadRules: err = %v, want nil", err)
	}
	if rules != nil {
		t.Errorf("LoadRules: rules = %v, want nil", rules)
	}
}

func TestLoadRules_emptyDir_returnsEmptyOrNilSlice(t *testing.T) {
	dir := t.TempDir()
	rules, err := LoadRules(dir)
	if err != nil {
		t.Errorf("LoadRules: err = %v", err)
	}
	// Empty dir yields no .mdc files, so out is nil or empty slice.
	if rules != nil && len(rules) != 0 {
		t.Errorf("len(rules) = %d, want 0", len(rules))
	}
}

func TestLoadRules_validMDC_loadsRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mdc")
	content := "---\nglobs: \"*.go\"\n---\nGo rule body.\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	rules, err := LoadRules(dir)
	if err != nil {
		t.Errorf("LoadRules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}
	if rules[0].Content != "Go rule body." {
		t.Errorf("Content = %q", rules[0].Content)
	}
	if len(rules[0].Globs) != 1 || rules[0].Globs[0] != "*.go" {
		t.Errorf("Globs = %v", rules[0].Globs)
	}
}

func TestLoadRules_skipNonMDC(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Hi"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "rule.mdc"), []byte("---\nglobs: \"*\"\n---\nBody"), 0644)
	rules, _ := LoadRules(dir)
	if len(rules) != 1 {
		t.Errorf("len(rules) = %d, want 1 (only .mdc)", len(rules))
	}
}

func TestLoadRules_invalidFile_skipped(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "bad.mdc"), []byte("---\nbroken: [\n---\nbody"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "good.mdc"), []byte("---\nglobs: \"*\"\n---\nOK"), 0644)
	rules, _ := LoadRules(dir)
	if len(rules) != 1 {
		t.Errorf("len(rules) = %d, want 1 (good only)", len(rules))
	}
	if rules[0].Content != "OK" {
		t.Errorf("Content = %q", rules[0].Content)
	}
}
