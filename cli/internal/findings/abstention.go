// Package findings defines the schema for code review findings: types, JSON
// contract, and validation. This file implements the Phase 6.3 abstention filter.

package findings

// Phase 6.3 abstention filter thresholds. Findings below these are discarded
// so low-confidence or maintainability-only suggestions (e.g. "add comments")
// do not appear in the final output.
const (
	// MinConfidenceKeep is the minimum confidence to keep any finding.
	// Findings with confidence < MinConfidenceKeep are dropped.
	MinConfidenceKeep = 0.8
	// MinConfidenceMaintainability is the minimum confidence for maintainability
	// findings. Maintainability findings with confidence < this are dropped.
	MinConfidenceMaintainability = 0.9
)

// FilterAbstention post-processes findings by dropping those that fail the
// abstention rules: (1) confidence < MinConfidenceKeep, or (2) category ==
// maintainability and confidence < MinConfidenceMaintainability. Returns a new
// slice; the input is not modified. Order of kept findings is preserved.
func FilterAbstention(list []Finding) []Finding {
	if len(list) == 0 {
		return nil
	}
	out := make([]Finding, 0, len(list))
	for _, f := range list {
		if f.Confidence < MinConfidenceKeep {
			continue
		}
		if f.Category == CategoryMaintainability && f.Confidence < MinConfidenceMaintainability {
			continue
		}
		out = append(out, f)
	}
	return out
}
