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
// The response may be a JSON array of finding objects, a wrapper object
// with a "findings" key, or a single finding object (corrective fallback when
// the model returns one finding as an object instead of an array).
// Each finding must have severity, category, and message. File may be omitted
// and filled later from the hunk; line and range are optional (file-only
// findings are valid). IDs are not set; use AssignFindingIDs after parsing.
func ParseFindingsResponse(jsonStr string) ([]findings.Finding, error) {
	jsonStr = trimSpace(jsonStr)
	if jsonStr == "" {
		return nil, fmt.Errorf("parse findings: empty response")
	}
	var raw []findings.Finding
	if err := json.Unmarshal([]byte(jsonStr), &raw); err == nil {
		return raw, nil
	}
	// Check for "findings" key to distinguish {"findings":[]} from a single finding object.
	var keyCheck map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &keyCheck); err != nil {
		return nil, fmt.Errorf("parse findings: %w", err)
	}
	if _, hasFindings := keyCheck["findings"]; hasFindings {
		var wrapper struct {
			Findings []findings.Finding `json:"findings"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &wrapper); err != nil {
			return nil, fmt.Errorf("parse findings: %w", err)
		}
		return wrapper.Findings, nil
	}
	// Corrective fallback: some models return a single finding object instead of an array or wrapper.
	var single findings.Finding
	if err := json.Unmarshal([]byte(jsonStr), &single); err != nil {
		return nil, fmt.Errorf("parse findings: %w", err)
	}
	if verr := single.Validate(); verr != nil {
		return nil, fmt.Errorf("parse findings: single object validation failed: %w", verr)
	}
	return []findings.Finding{single}, nil
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
		if f.Confidence == 0 {
			f.Confidence = 1.0
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
