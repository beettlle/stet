// Package review implements review orchestration: prompt loading, Ollama generate,
// response parsing, and tool-generated finding IDs. Used by the run flow (Phase 3.5).
package review

import (
	"encoding/json"
	"fmt"

	"stet/cli/internal/findings"
	"stet/cli/internal/hunkid"
)

// ParseFindingsResponse unmarshals the LLM response into a slice of findings.
// The response may be a JSON array of finding objects or a wrapper object
// with a "findings" key. Each finding must have severity, category, and message.
// File may be omitted and filled later from the hunk; line and range are optional
// (file-only findings are valid). IDs are not set; use AssignFindingIDs after parsing.
func ParseFindingsResponse(jsonStr string) ([]findings.Finding, error) {
	jsonStr = trimSpace(jsonStr)
	if jsonStr == "" {
		return nil, fmt.Errorf("parse findings: empty response")
	}
	var raw []findings.Finding
	if err := json.Unmarshal([]byte(jsonStr), &raw); err == nil {
		return raw, nil
	}
	var wrapper struct {
		Findings []findings.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &wrapper); err != nil {
		return nil, fmt.Errorf("parse findings: %w", err)
	}
	return wrapper.Findings, nil
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

// AssignFindingIDs sets ID on each finding using StableFindingID and validates.
// If a finding has an empty File, hunkFilePath is used (so the finding is valid).
// Returns error on first validation failure.
func AssignFindingIDs(list []findings.Finding, hunkFilePath string) ([]findings.Finding, error) {
	out := make([]findings.Finding, 0, len(list))
	for i := range list {
		f := list[i]
		if f.File == "" {
			f.File = hunkFilePath
		}
		rangeStart, rangeEnd := 0, 0
		if f.Range != nil {
			rangeStart, rangeEnd = f.Range.Start, f.Range.End
		}
		f.ID = hunkid.StableFindingID(f.File, f.Line, rangeStart, rangeEnd, f.Message)
		if verr := f.Validate(); verr != nil {
			return nil, fmt.Errorf("finding %d: %w", i, verr)
		}
		out = append(out, f)
	}
	return out, nil
}
