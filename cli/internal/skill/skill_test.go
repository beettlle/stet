package skill

import (
	"strings"
	"testing"
)

func TestSKILL_containsFrontmatter(t *testing.T) {
	t.Parallel()
	content := SKILL("1.2.3")
	if !strings.Contains(content, "---") {
		t.Error("SKILL output must contain YAML frontmatter delimiter ---")
	}
	if !strings.Contains(content, "name: stet-integration") {
		t.Error("SKILL output must contain name: stet-integration")
	}
	if !strings.Contains(content, "description:") {
		t.Error("SKILL output must contain description")
	}
	if !strings.Contains(content, "metadata:") {
		t.Error("SKILL output must contain metadata")
	}
	if !strings.Contains(content, "version: \"1.2.3\"") {
		t.Error("SKILL output must contain version: \"1.2.3\" from argument")
	}
}

func TestSKILL_containsRequiredSections(t *testing.T) {
	t.Parallel()
	content := SKILL("dev")
	for _, section := range []string{
		"# Stet Integration",
		"## When to Use This Skill",
		"## Commands",
		"## Dismiss Reasons",
		"## Rules",
		"## Examples",
		"stet doctor",
		"stet start",
		"stet dismiss",
		"false_positive",
		"already_correct",
		"wrong_suggestion",
		"out_of_scope",
	} {
		if !strings.Contains(content, section) {
			t.Errorf("SKILL output must contain %q", section)
		}
	}
}

func TestSKILL_emptyVersionDefaults(t *testing.T) {
	t.Parallel()
	content := SKILL("")
	if !strings.Contains(content, "version: \"1.0\"") {
		t.Error("SKILL(\"\") should embed version 1.0 as default")
	}
}
