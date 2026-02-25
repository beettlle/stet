// Package rules loads and filters Cursor rule files (.mdc) for injection into
// the review system prompt. Used by Sub-phase 6.6.
package rules

import (
	"os"
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
	// MaxRuleTokens is the approximate token budget for combined rule content
	// injected into the system prompt. Content is truncated from the end when exceeded.
	MaxRuleTokens = 1000
)

// CursorRule represents a single .mdc rule: frontmatter (globs, alwaysApply,
// description) and body content to inject when the rule applies to the file under review.
// Source is the rule file name (e.g. general-llm-anti-patterns.mdc) for trace and debugging.
type CursorRule struct {
	Globs       []string
	AlwaysApply bool
	Description string
	Content     string
	Source      string
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
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, nil
	}
	var out []CursorRule
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".mdc") {
			continue
		}
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
		out = append(out, rule)
	}
	return out, nil
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
	targetSlash := filepath.ToSlash(targetFile)
	base := filepath.Base(targetFile)
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
			patSlash := filepath.ToSlash(pat)
			var name string
			if strings.Contains(patSlash, "/") {
				name = targetSlash
			} else {
				name = base
			}
			ok, err := filepath.Match(patSlash, name)
			if err == nil && ok {
				out = append(out, r)
				break
			}
		}
	}
	return out
}
