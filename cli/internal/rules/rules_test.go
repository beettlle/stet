package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func mustCreateRulesDir(t *testing.T, base, rel string) string {
	t.Helper()
	p := filepath.Join(base, rel, ".cursor", "rules")
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	return p
}

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
	if rules[0].Source != "test.mdc" {
		t.Errorf("Source = %q, want test.mdc (rule file name)", rules[0].Source)
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
	if rules[0].Source != "rule.mdc" {
		t.Errorf("Source = %q, want rule.mdc", rules[0].Source)
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
	if rules[0].Source != "good.mdc" {
		t.Errorf("Source = %q, want good.mdc", rules[0].Source)
	}
}

func TestParseMDC_descriptionOnly_infersGlobs(t *testing.T) {
	content := "---\ndescription: \"TypeScript and frontend components\"\n---\nUse TS in frontend.\n"
	rule, ok := parseMDC(content)
	if !ok {
		t.Fatal("parseMDC: want true")
	}
	if rule.Description != "TypeScript and frontend components" {
		t.Errorf("Description = %q", rule.Description)
	}
	if len(rule.Globs) == 0 {
		t.Fatal("Globs: want inferred from description, got empty")
	}
	// Should include *.ts, *.tsx (TypeScript/ts) and frontend/* (frontend)
	hasTS := false
	hasFrontend := false
	for _, g := range rule.Globs {
		if g == "*.ts" || g == "*.tsx" {
			hasTS = true
		}
		if g == "frontend/*" {
			hasFrontend = true
		}
	}
	if !hasTS {
		t.Errorf("Globs = %v: want *.ts or *.tsx from TypeScript", rule.Globs)
	}
	if !hasFrontend {
		t.Errorf("Globs = %v: want frontend/* from frontend", rule.Globs)
	}
}

func TestParseMDC_descriptionNoKeyword_globsStayNil(t *testing.T) {
	content := "---\ndescription: \"Miscellaneous coding style\"\n---\nBody.\n"
	rule, ok := parseMDC(content)
	if !ok {
		t.Fatal("parseMDC: want true")
	}
	if rule.Globs != nil {
		t.Errorf("Globs = %v, want nil when no keyword match", rule.Globs)
	}
}

func TestFilterRules_inferredGlobs_matchIncluded(t *testing.T) {
	rule, _ := parseMDC("---\ndescription: \"Go\"\n---\nGo rule.\n")
	if len(rule.Globs) == 0 {
		t.Fatal("expected inferred globs for Go")
	}
	matched := FilterRules([]CursorRule{rule}, "cli/main.go")
	if len(matched) != 1 {
		t.Errorf("len(matched) = %d, want 1", len(matched))
	}
	if matched[0].Content != "Go rule." {
		t.Errorf("Content = %q", matched[0].Content)
	}
}

func TestFilterRules_inferredGlobs_noMatchExcluded(t *testing.T) {
	rule, _ := parseMDC("---\ndescription: \"Go\"\n---\nGo rule.\n")
	matched := FilterRules([]CursorRule{rule}, "app.ts")
	if len(matched) != 0 {
		t.Errorf("len(matched) = %d, want 0 for app.ts", len(matched))
	}
}

func TestInferGlobsFromDescription_emptyReturnsNil(t *testing.T) {
	got := InferGlobsFromDescription("")
	if got != nil {
		t.Errorf("InferGlobsFromDescription(\"\") = %v, want nil", got)
	}
}

func TestInferGlobsFromDescription_noMatchReturnsNil(t *testing.T) {
	got := InferGlobsFromDescription("miscellaneous style guide")
	if got != nil {
		t.Errorf("InferGlobsFromDescription(no keyword) = %v, want nil", got)
	}
}

func TestInferGlobsFromDescription_singleKeyword(t *testing.T) {
	got := InferGlobsFromDescription("Python")
	if len(got) == 0 {
		t.Fatal("want at least one glob for Python")
	}
	found := false
	for _, g := range got {
		if g == "*.py" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("InferGlobsFromDescription(\"Python\") = %v, want *.py", got)
	}
}

func TestDiscoverRulesDirs_rootAndNested(t *testing.T) {
	repo := t.TempDir()
	mustCreateRulesDir(t, repo, "")
	mustCreateRulesDir(t, repo, "cli")
	mustCreateRulesDir(t, repo, "extension")
	_ = os.WriteFile(filepath.Join(repo, ".cursor", "rules", "a.mdc"), []byte("---\nalwaysApply: true\n---\nRoot"), 0644)
	_ = os.WriteFile(filepath.Join(repo, "cli", ".cursor", "rules", "b.mdc"), []byte("---\nglobs: \"*.go\"\n---\nCLI Go"), 0644)
	_ = os.WriteFile(filepath.Join(repo, "extension", ".cursor", "rules", "c.mdc"), []byte("---\nglobs: \"*.ts\"\n---\nExt TS"), 0644)

	dirs, err := DiscoverRulesDirs(repo)
	if err != nil {
		t.Fatalf("DiscoverRulesDirs: %v", err)
	}
	if len(dirs) != 3 {
		t.Fatalf("len(dirs) = %d, want 3", len(dirs))
	}
	relPaths := make(map[string]bool)
	for _, d := range dirs {
		relPaths[d.RelPath] = true
	}
	if !relPaths[""] {
		t.Error("want root RelPath \"\"")
	}
	if !relPaths["cli"] {
		t.Error("want RelPath \"cli\"")
	}
	if !relPaths["extension"] {
		t.Error("want RelPath \"extension\"")
	}
	// Root should be first (sorted)
	if dirs[0].RelPath != "" {
		t.Errorf("first dir RelPath = %q, want \"\"", dirs[0].RelPath)
	}
}

func TestLoader_RulesForFile_prefixMatch(t *testing.T) {
	repo := t.TempDir()
	mustCreateRulesDir(t, repo, "")
	mustCreateRulesDir(t, repo, "cli")
	mustCreateRulesDir(t, repo, "extension")
	_ = os.WriteFile(filepath.Join(repo, ".cursor", "rules", "root.mdc"), []byte("---\nalwaysApply: true\n---\nRoot rule"), 0644)
	_ = os.WriteFile(filepath.Join(repo, "cli", ".cursor", "rules", "go.mdc"), []byte("---\nglobs: \"*.go\"\n---\nCLI Go"), 0644)
	_ = os.WriteFile(filepath.Join(repo, "extension", ".cursor", "rules", "ts.mdc"), []byte("---\nglobs: \"*.ts\"\n---\nExt TS"), 0644)

	loader := NewLoader(repo)

	// cli/internal/run/run.go -> root + cli (not extension)
	rules := loader.RulesForFile("cli/internal/run/run.go")
	if len(rules) != 2 {
		t.Fatalf("cli file: len(rules) = %d, want 2", len(rules))
	}
	contents := make(map[string]bool)
	for _, r := range rules {
		contents[r.Content] = true
	}
	if !contents["Root rule"] {
		t.Error("want Root rule for cli file")
	}
	if !contents["CLI Go"] {
		t.Error("want CLI Go for cli file")
	}
	if contents["Ext TS"] {
		t.Error("should not include Ext TS for cli file")
	}

	// extension/src/foo.ts -> root + extension
	rules = loader.RulesForFile("extension/src/foo.ts")
	if len(rules) != 2 {
		t.Fatalf("extension file: len(rules) = %d, want 2", len(rules))
	}
	contents = make(map[string]bool)
	for _, r := range rules {
		contents[r.Content] = true
	}
	if !contents["Ext TS"] {
		t.Error("want Ext TS for extension file")
	}

	// top.go -> root only
	rules = loader.RulesForFile("top.go")
	if len(rules) != 1 {
		t.Fatalf("top.go: len(rules) = %d, want 1", len(rules))
	}
	if rules[0].Content != "Root rule" {
		t.Errorf("top.go: Content = %q", rules[0].Content)
	}
}

func TestLoader_RulesForFile_mergeAndFilter(t *testing.T) {
	repo := t.TempDir()
	mustCreateRulesDir(t, repo, "")
	mustCreateRulesDir(t, repo, "cli")
	_ = os.WriteFile(filepath.Join(repo, ".cursor", "rules", "global.mdc"), []byte("---\nalwaysApply: true\n---\nGlobal"), 0644)
	_ = os.WriteFile(filepath.Join(repo, "cli", ".cursor", "rules", "go.mdc"), []byte("---\nglobs: \"*.go\"\n---\nGo only"), 0644)

	loader := NewLoader(repo)
	// cli/main.go: both rules included in merged list; FilterRules (in prompt) keeps both
	rules := loader.RulesForFile("cli/main.go")
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
	matched := FilterRules(rules, "cli/main.go")
	if len(matched) != 2 {
		t.Fatalf("FilterRules(cli/main.go): len = %d, want 2", len(matched))
	}
	// extension/main.ts: only root; Go rule does not match
	rules = loader.RulesForFile("extension/main.ts")
	matched = FilterRules(rules, "extension/main.ts")
	if len(matched) != 1 {
		t.Fatalf("FilterRules(extension/main.ts): len = %d, want 1", len(matched))
	}
	if matched[0].Content != "Global" {
		t.Errorf("matched[0].Content = %q", matched[0].Content)
	}
}

func TestLoader_noRulesDir_returnsNil(t *testing.T) {
	repo := t.TempDir()
	// No .cursor/rules at all
	loader := NewLoader(repo)
	rules := loader.RulesForFile("any/file.go")
	if rules != nil {
		t.Errorf("RulesForFile with no rules dir = %v, want nil", rules)
	}
}
