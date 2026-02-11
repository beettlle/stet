// Package findings defines the schema for code review findings: types, JSON
// contract, and validation. It is the single source of truth for the extension
// and CLI output.
package findings

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
	CategoryPerformance     Category = "performance"
	CategoryStyle           Category = "style"
	CategoryMaintainability  Category = "maintainability"
	CategoryTesting          Category = "testing"
)

// LineRange represents a span of lines (start and end inclusive).
type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Finding is a single code review finding with a stable id, location, severity,
// category, message, and optional suggestion and cursor URI.
type Finding struct {
	ID         string     `json:"id,omitempty"`
	File       string     `json:"file"`
	Line       int        `json:"line,omitempty"`
	Range      *LineRange `json:"range,omitempty"`
	Severity   Severity   `json:"severity"`
	Category   Category   `json:"category"`
	Message    string     `json:"message"`
	Suggestion string     `json:"suggestion,omitempty"`
	CursorURI  string     `json:"cursor_uri,omitempty"`
}
