package rules

import (
	"strings"
	"testing"
)

func TestBuildDigest(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "keeps_bad_and_detect_lines_per_section",
			content: "## Persona\nBe skeptical.\n\n### 1.1 Ghost Layer\n**CRITICAL:** prose\n❌ Bad: `return repo.get()`\n✅ Good: add logic\n**Detect:** >80% delegation\n\n### 1.2 Fake Abstraction\n❌ Bad: actor hop\n**Detect:** escapes to main",
			want:    "### 1.1 Ghost Layer\n❌ Bad: `return repo.get()`\n**Detect:** >80% delegation\n\n### 1.2 Fake Abstraction\n❌ Bad: actor hop\n**Detect:** escapes to main",
		},
		{
			name:    "drops_sections_without_markers",
			content: "## Communication\nLead with outcome.\n\n### 2.1 Loops\n**Detect:** I/O in loop",
			want:    "### 2.1 Loops\n**Detect:** I/O in loop",
		},
		{
			name:    "ignores_markers_inside_fences",
			content: "### 3.1 Example\n```\n❌ Bad: inside fence\n**Detect:** inside fence\n```\n**Detect:** outside fence",
			want:    "### 3.1 Example\n**Detect:** outside fence",
		},
		{
			name:    "no_markers_falls_back_to_full_content",
			content: "## Style\nUse tabs.",
			want:    "## Style\nUse tabs.",
		},
		{
			name:    "preamble_markers_use_source_heading",
			content: "**Detect:** preamble check\n\n## Other\nprose",
			want:    "### src.mdc\n**Detect:** preamble check",
		},
		{
			name:    "empty_content",
			content: "",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDigest(CursorRule{Source: "src.mdc", Content: tt.content})
			if got != tt.want {
				t.Errorf("BuildDigest() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestBuildDigest_resplitsIntoSameSections(t *testing.T) {
	r := CursorRule{Source: "x.mdc", Content: "### A\n**Detect:** a\n\n### B\n❌ Bad: b"}
	sections := SplitSections(CursorRule{Source: r.Source, Content: BuildDigest(r)})
	if len(sections) != 2 || sections[0].Heading != "A" || sections[1].Heading != "B" {
		t.Fatalf("SplitSections(digest) = %+v, want headings A, B", sections)
	}
}

func TestDigestOf_prefersLoadedDigest(t *testing.T) {
	if got := digestOf(CursorRule{Digest: "cached", Content: "### H\n**Detect:** x"}); got != "cached" {
		t.Errorf("digestOf(with Digest) = %q, want cached", got)
	}
	if got := digestOf(CursorRule{Content: "### H\n**Detect:** x"}); !strings.Contains(got, "**Detect:** x") {
		t.Errorf("digestOf(without Digest) = %q, want built digest", got)
	}
}
