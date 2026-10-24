// Package findings defines the schema for code review findings: types, JSON
// contract, and validation. It is the single source of truth for the extension
// and CLI output.
package findings

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Severity is the severity level of a finding.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
	SeverityNitpick Severity = "nitpick"
)

// Rank orders severities for threshold filtering: error (4) > warning (3) >
// info (2) > nitpick (1). Unknown severities rank 0.
func (s Severity) Rank() int {
	switch s {
	case SeverityError:
		return 4
	case SeverityWarning:
		return 3
	case SeverityInfo:
		return 2
	case SeverityNitpick:
		return 1
	default:
		return 0
	}
}

// ParseSeverity returns the Severity named by s (case-insensitive, trimmed)
// and whether s names a known severity.
func ParseSeverity(s string) (Severity, bool) {
	sev := Severity(strings.ToLower(strings.TrimSpace(s)))
	if sev.Rank() == 0 {
		return "", false
	}
	return sev, true
}

// FilterAtLeast returns the findings whose severity ranks at least minSeverity
// and whose confidence is at least minConfidence. An empty minSeverity or a
// zero minConfidence disables that check. Order is preserved; the input is not
// modified.
func FilterAtLeast(list []Finding, minSeverity Severity, minConfidence float64) []Finding {
	minRank := minSeverity.Rank()
	out := make([]Finding, 0, len(list))
	for _, f := range list {
		if f.Severity.Rank() < minRank {
			continue
		}
		if f.Confidence < minConfidence {
			continue
		}
		out = append(out, f)
	}
	return out
}

// Category is the category of a finding for filtering and display.
type Category string

const (
	CategoryBug             Category = "bug"
	CategorySecurity        Category = "security"
	CategoryCorrectness     Category = "correctness"
	CategoryPerformance     Category = "performance"
	CategoryStyle           Category = "style"
	CategoryMaintainability Category = "maintainability"
	CategoryBestPractice    Category = "best_practice"
	CategoryTesting         Category = "testing"
	CategoryDocumentation   Category = "documentation"
	CategoryDesign          Category = "design"
	CategoryAccessibility   Category = "accessibility"
)

// LineRange represents a span of lines (start and end inclusive).
type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// EvidenceLines is a slice of line numbers that support the finding. It
// unmarshals from either a JSON array of integers or a comma-separated string,
// so model output that uses a string for evidence_lines does not break parsing.
type EvidenceLines []int

// UnmarshalJSON implements json.Unmarshaler. It accepts an array of integers
// or a single string of comma-separated line numbers (e.g. "10, 12").
func (e *EvidenceLines) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*e = nil
		return nil
	}
	var arr []int
	if err := json.Unmarshal(data, &arr); err == nil {
		*e = arr
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		*e = nil
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return err
		}
		out = append(out, n)
	}
	*e = out
	return nil
}

// RuleConflict records the project guideline section a finding's suggestion
// most likely conflicts with, as judged by an optional System One choice
// annotation (review.AnnotateRuleConflicts). Human-visible only: findings are
// never dropped, reordered, or rescored based on it.
type RuleConflict struct {
	// Source is the rule file the conflicting section came from
	// (e.g. "go-1-21-development-standards.mdc").
	Source string `json:"source"`
	// Section is the section heading within the rule file.
	Section string `json:"section"`
	// Probability is P(choice) from the System One decision.
	Probability float64 `json:"probability"`
}

// Finding is a single code review finding with a stable id, location, severity,
// category, confidence, message, and optional suggestion and cursor URI.
type Finding struct {
	ID            string        `json:"id,omitempty"`
	File          string        `json:"file"`
	Line          int           `json:"line,omitempty"`
	Range         *LineRange    `json:"range,omitempty"`
	Severity      Severity      `json:"severity"`
	Category      Category      `json:"category"`
	Confidence    float64       `json:"confidence"`
	Message       string        `json:"message"`
	Suggestion    string        `json:"suggestion,omitempty"`
	CursorURI     string        `json:"cursor_uri,omitempty"`
	EvidenceLines EvidenceLines `json:"evidence_lines,omitempty"`
	// RuleConflict is the optional rules-compliance annotation; nil unless a
	// System One annotation pass judged a guideline conflict above threshold.
	RuleConflict *RuleConflict `json:"rule_conflict,omitempty"`
}
