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

// Finding is a single code review finding with a stable id, location, severity,
// category, confidence, message, and optional suggestion and cursor URI.
type Finding struct {
	ID         string     `json:"id,omitempty"`
	File       string     `json:"file"`
	Line       int        `json:"line,omitempty"`
	Range      *LineRange `json:"range,omitempty"`
	Severity   Severity   `json:"severity"`
	Category   Category   `json:"category"`
	Confidence float64     `json:"confidence"`
	Message    string     `json:"message"`
	Suggestion    string     `json:"suggestion,omitempty"`
	CursorURI     string     `json:"cursor_uri,omitempty"`
	EvidenceLines EvidenceLines `json:"evidence_lines,omitempty"`
}

