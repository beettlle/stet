// Package history defines the schema for .review/history.jsonl, the
// optimizer and prompt-shadowing feedback log. Each line is a single JSON
// object (Record). The schema supports optional per-finding dismissal
// reasons so the DSPy optimizer and prompt shadowing can learn which
// patterns to avoid. When Phase 4.5 is implemented, the CLI will append
// records on user actions (e.g. dismiss and/or finish with findings).
// Bounded size or rotation (e.g. last N sessions) should be applied to
// avoid unbounded growth. Schema is designed for future export/upload
// for org-wide aggregation.
package history

import "stet/cli/internal/findings"

// Dismissal reason constants. Used when a finding is dismissed or marked
// not acted on; the optimizer uses these to down-weight similar patterns.
// ReasonAlreadyCorrect is also used when a finding is auto-dismissed because
// a re-review of the same code (e.g. after the user fixed the issue) did not report it.
const (
	ReasonFalsePositive   = "false_positive"
	ReasonAlreadyCorrect  = "already_correct"
	ReasonWrongSuggestion = "wrong_suggestion"
	ReasonOutOfScope      = "out_of_scope"
)

// ValidReason returns true if s is one of the allowed dismissal reason constants.
func ValidReason(s string) bool {
	switch s {
	case ReasonFalsePositive, ReasonAlreadyCorrect, ReasonWrongSuggestion, ReasonOutOfScope:
		return true
	default:
		return false
	}
}

// Dismissal represents one finding dismissed with an optional reason.
type Dismissal struct {
	FindingID string `json:"finding_id"`
	Reason    string `json:"reason,omitempty"` // One of Reason* constants, or empty.
}

// UserAction holds feedback from the user: which findings were dismissed
// (with optional reasons) and when the session finished.
// ReplaceFindings, when true, indicates the session findings were replaced
// by a rerun with --replace (not merged); used for audit trail.
type UserAction struct {
	DismissedIDs    []string    `json:"dismissed_ids,omitempty"`
	Dismissals      []Dismissal `json:"dismissals,omitempty"`       // Per-finding reason when present.
	FinishedAt      string      `json:"finished_at,omitempty"`      // ISO8601 or similar.
	ReplaceFindings bool        `json:"replace_findings,omitempty"` // True when rerun --replace replaced session findings.
}

// Record is one line in .review/history.jsonl.
type Record struct {
	DiffRef      string             `json:"diff_ref"`      // Ref or SHA for the diff scope.
	ReviewOutput []findings.Finding `json:"review_output"` // Findings from the review run.
	UserAction   UserAction         `json:"user_action"`
}
