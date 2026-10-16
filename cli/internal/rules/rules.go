// Package rules loads and filters Cursor rule files (.mdc) for injection into
// the review system prompt. Used by Sub-phase 6.6.
package rules

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var descriptionTokenSplit = regexp.MustCompile(`[^a-z0-9]+`)

// descriptionGlobMap maps description keywords to glob patterns (filepath.Match compatible, no **).
var descriptionGlobMap = map[string][]string{
	"typescript": {"*.ts", "*.tsx"}, "ts": {"*.ts", "*.tsx"},
	"go": {"*.go"}, "golang": {"*.go"},
	"python": {"*.py"}, "py": {"*.py"},
	"react":      {"*.tsx", "*.jsx"},
	"javascript": {"*.js", "*.jsx"}, "js": {"*.js", "*.jsx"},
	"frontend": {"frontend/*"}, "backend": {"backend/*"},
	"cli": {"cli/*"}, "extension": {"extension/*"},
	"api": {"api/*"},
}

const (
	// MaxRuleTokens is the approximate token budget for rule sections chosen by
	// the rule selector, and the floor of ReviewRuleBudget for the default path.
	MaxRuleTokens = 1000
)

// CursorRule represents a single .mdc rule: frontmatter (globs, alwaysApply,
// description) and body content to inject when the rule applies to the file under review.
// Source is the rule file name (e.g. general-llm-anti-patterns.mdc) for trace and debugging.
// Path is the rule file's path as loaded. Digest is the review digest (see BuildDigest)
// filled by the loader; when empty it is built on demand.
type CursorRule struct {
	Globs       []string
	AlwaysApply bool
	Description string
	Content     string
	Source      string
	Path        string
	Digest      string
}

// frontmatter is the YAML structure we parse from .mdc files.
type frontmatter struct {
	Globs       interface{} `yaml:"globs"` // string or []string
	AlwaysApply bool        `yaml:"alwaysApply"`
	Description string      `yaml:"description"`
}

// LoadRules reads all .mdc files from rulesDir and returns parsed rules.
// If rulesDir does not exist, returns (nil, nil) with no error so stet works
// without any Cursor rules. On per-file read/parse errors, that file is
// skipped; the function returns collected rules and nil error.
func LoadRules(rulesDir string) ([]CursorRule, error) {
	loaded, _ := loadRulesDir(rulesDir, nil)
	return loaded, nil
}

// loadRulesDir reads and parses every .mdc file in rulesDir, fills each rule's
// Path and Digest (through digests when non-nil), and returns the rules with
// the directory fingerprint observed while loading (see dirFingerprint).
// Unreadable directories yield no rules; unreadable or invalid files are skipped.
func loadRulesDir(rulesDir string, digests *DigestCache) ([]CursorRule, string) {
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		return nil, ""
	}
	var out []CursorRule
	var fp strings.Builder
	for _, e := range entries {
		if !isRuleFile(e) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		writeStamp(&fp, e.Name(), info)
		path := filepath.Join(rulesDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		rule, ok := parseMDC(string(data))
		if !ok {
			continue
		}
		rule.Source = e.Name()
		rule.Path = path
		rule.Digest = digests.digestFor(rule, info.Size(), info.ModTime().UnixNano(), data)
		out = append(out, rule)
	}
	return out, fp.String()
}

// parseMDC splits content on --- delimiters, parses YAML frontmatter, and
// returns the rule with body as Content. Returns (zero value, false) on failure.
func parseMDC(content string) (CursorRule, bool) {
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 2 {
		return CursorRule{}, false
	}
	var fm frontmatter
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(parts[1])), &fm); err != nil {
		return CursorRule{}, false
	}
	globs := normalizeGlobs(fm.Globs)
	body := ""
	if len(parts) == 3 {
		body = strings.TrimSpace(parts[2])
	}
	rule := CursorRule{
		Globs:       globs,
		AlwaysApply: fm.AlwaysApply,
		Description: strings.TrimSpace(fm.Description),
		Content:     body,
	}
	if len(rule.Globs) == 0 && !rule.AlwaysApply && rule.Description != "" {
		rule.Globs = InferGlobsFromDescription(rule.Description)
	}
	return rule, true
}

func normalizeGlobs(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil
		}
		return []string{s}
	case []interface{}:
		var out []string
		for _, item := range x {
			if s, ok := item.(string); ok {
				t := strings.TrimSpace(s)
				if t != "" {
					out = append(out, t)
				}
			}
		}
		return out
	default:
		return nil
	}
}

// InferGlobsFromDescription infers glob patterns from a rule's description when the rule
// has no explicit globs. Tokenizes the description (lowercase, non-alphanumeric split),
// matches tokens against a fixed keyword table, and returns the union of matched globs.
// Returns nil if no keywords match. Patterns are filepath.Match compatible (no **).
func InferGlobsFromDescription(description string) []string {
	if description == "" {
		return nil
	}
	tokens := descriptionTokenSplit.Split(strings.ToLower(description), -1)
	var seen map[string]bool
	var out []string
	for _, t := range tokens {
		if t == "" {
			continue
		}
		if globs, ok := descriptionGlobMap[t]; ok {
			for _, g := range globs {
				if !seen[g] {
					if seen == nil {
						seen = make(map[string]bool)
					}
					seen[g] = true
					out = append(out, g)
				}
			}
		}
	}
	return out
}

// FilterRules returns rules that apply to targetFile: either AlwaysApply is true
// or at least one glob matches. targetFile is a path relative to repo root
// (e.g. "app.ts" or "foo/bar.js"). Glob patterns use forward slashes for matching.
func FilterRules(rules []CursorRule, targetFile string) []CursorRule {
	if len(rules) == 0 {
		return nil
	}
	var out []CursorRule
	for _, r := range rules {
		if r.AlwaysApply {
			out = append(out, r)
			continue
		}
		if len(r.Globs) == 0 {
			continue
		}
		for _, pat := range r.Globs {
			if matchGlob(pat, targetFile) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// matchGlob reports whether target (a repo-root-relative file path) matches
// pattern using gitignore/Cursor-style semantics with forward slashes:
//
//   - "**" as a full path segment matches zero or more path segments:
//     "**/*.go" matches "main.go" and "cli/internal/run/run.go"; "cli/**"
//     matches everything under "cli/"; "cli/**/x_test.go" matches both
//     "cli/x_test.go" and "cli/a/b/x_test.go".
//   - Patterns without "/" match the base name of target ("*.ts" matches "src/a.ts").
//   - Patterns with "/" but no "**" keep filepath.Match semantics against the
//     full path.
//
// Malformed patterns (e.g. "[") return false instead of panicking. Only the
// standard library is used; "**" inside a segment (not a full segment, e.g.
// "cli**") degrades to filepath.Match wildcard behaviour for that segment.
func matchGlob(pattern, target string) bool {
	pat := filepath.ToSlash(pattern)
	tgt := filepath.ToSlash(target)
	if !strings.Contains(pat, "/") {
		ok, err := path.Match(pat, path.Base(tgt))
		return err == nil && ok
	}
	if !strings.Contains(pat, "**") {
		ok, err := path.Match(pat, tgt)
		return err == nil && ok
	}
	return matchSegments(strings.Split(pat, "/"), strings.Split(tgt, "/"))
}

// matchSegments matches pattern segments against target segments (both already
// slash-separated). A "**" pattern segment matches zero or more target
// segments; every other segment is matched with path.Match. Returns false for
// malformed pattern segments (path.Match error).
func matchSegments(pat, tgt []string) bool {
	if len(pat) == 0 {
		return len(tgt) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(tgt); i++ {
			if matchSegments(pat[1:], tgt[i:]) {
				return true
			}
		}
		return false
	}
	if len(tgt) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], tgt[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pat[1:], tgt[1:])
}
