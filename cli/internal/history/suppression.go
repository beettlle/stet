// Suppression builds example strings from history dismissals for prompt injection
// (Dynamic Suppression / roadmap 9.1). Used when building the system prompt so
// the model avoids reporting issues similar to previously dismissed findings.
package history

import (
	"fmt"
	"strings"

	"stet/cli/internal/findings"
)

// SuppressionExamples reads history from stateDir, takes the last maxRecords
// records, and extracts one short example string per dismissal that can be
// resolved to a finding in the same record's ReviewOutput. Examples are
// formatted as "file:line: message", deduplicated (normalized: trim, collapse
// whitespace), and capped at maxExamples (oldest dropped). Returns nil, nil
// on missing/empty history or no resolvable dismissals (fail open: no section).
// On read error (e.g. directory unreadable), returns nil, err.
func SuppressionExamples(stateDir string, maxRecords, maxExamples int) ([]string, error) {
	records, err := ReadRecords(stateDir)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	if maxRecords <= 0 || maxExamples <= 0 {
		return nil, nil
	}
	// Last maxRecords (oldest to newest in slice).
	start := 0
	if len(records) > maxRecords {
		start = len(records) - maxRecords
	}
	slice := records[start:]
	var raw []string
	seen := make(map[string]struct{})
	for _, rec := range slice {
		if len(rec.ReviewOutput) == 0 || len(rec.UserAction.Dismissals) == 0 {
			continue
		}
		byID := make(map[string]findings.Finding)
		for _, f := range rec.ReviewOutput {
			if f.ID != "" {
				byID[f.ID] = f
			}
		}
		for _, d := range rec.UserAction.Dismissals {
			if d.FindingID == "" {
				continue
			}
			f, ok := byID[d.FindingID]
			if !ok {
				continue
			}
			ex := formatExample(f)
			norm := normalizeExample(ex)
			if norm == "" {
				continue
			}
			if _, ok := seen[norm]; ok {
				continue
			}
			seen[norm] = struct{}{}
			raw = append(raw, ex)
		}
	}
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) <= maxExamples {
		return raw, nil
	}
	// Keep newest maxExamples (drop oldest from front).
	return raw[len(raw)-maxExamples:], nil
}

func formatExample(f findings.Finding) string {
	if f.File == "" && f.Message == "" {
		return ""
	}
	if f.File == "" {
		return f.Message
	}
	if f.Line <= 0 {
		return f.File + ": " + f.Message
	}
	return fmt.Sprintf("%s:%d: %s", f.File, f.Line, f.Message)
}

func normalizeExample(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Fields(s)
	return strings.Join(parts, " ")
}
