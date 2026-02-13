// Package findings defines the schema for code review findings: types, JSON
// contract, and validation. This file implements the Phase 6.3 abstention filter.

package findings

// FilterAbstention post-processes findings by dropping those that fail the
// abstention rules: (1) confidence < minKeep, or (2) category ==
// maintainability and confidence < minMaint. Returns a new slice; the input
// is not modified. Order of kept findings is preserved.
// Callers typically use findings.ResolveStrictness(cfg.Strictness) to obtain
// minKeep and minMaint; default thresholds are 0.8 and 0.9.
func FilterAbstention(list []Finding, minKeep, minMaint float64) []Finding {
	if len(list) == 0 {
		return nil
	}
	out := make([]Finding, 0, len(list))
	for _, f := range list {
		if f.Confidence < minKeep {
			continue
		}
		if f.Category == CategoryMaintainability && f.Confidence < minMaint {
			continue
		}
		out = append(out, f)
	}
	return out
}
