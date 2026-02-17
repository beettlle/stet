// Package stats provides impact reporting (volume, quality, energy) from
// stet git notes and history.
package stats

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"stet/cli/internal/git"
)

// CloudModel holds pricing for cloud cost estimation (per-million tokens).
type CloudModel struct {
	Name          string  `json:"name"`
	InPerMillion  float64 `json:"in_per_million"`
	OutPerMillion float64 `json:"out_per_million"`
}

// Built-in presets for cloud model pricing (approximate 2024 API rates).
var cloudModelPresets = map[string]CloudModel{
	"claude-sonnet": {Name: "claude-sonnet", InPerMillion: 3, OutPerMillion: 15},
	"gpt-4o-mini":   {Name: "gpt-4o-mini", InPerMillion: 0.15, OutPerMillion: 0.60},
}

// ParseCloudModel parses a cloud model spec: "NAME" (preset) or "NAME:in:out" (custom per-million).
// Returns an error for unknown presets or invalid custom format.
func ParseCloudModel(s string) (CloudModel, error) {
	if s == "" {
		return CloudModel{}, fmt.Errorf("cloud model spec is empty")
	}
	if idx := strings.Index(s, ":"); idx >= 0 {
		parts := strings.SplitN(s, ":", 3)
		if len(parts) != 3 {
			return CloudModel{}, fmt.Errorf("invalid cloud model spec %q: expected NAME:in:out", s)
		}
		name := strings.TrimSpace(parts[0])
		if name == "" {
			return CloudModel{}, fmt.Errorf("invalid cloud model spec %q: name is empty", s)
		}
		in, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return CloudModel{}, fmt.Errorf("invalid cloud model spec %q: in must be a number", s)
		}
		out, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		if err != nil {
			return CloudModel{}, fmt.Errorf("invalid cloud model spec %q: out must be a number", s)
		}
		if in < 0 || out < 0 {
			return CloudModel{}, fmt.Errorf("invalid cloud model spec %q: in and out must be non-negative", s)
		}
		return CloudModel{Name: name, InPerMillion: in, OutPerMillion: out}, nil
	}
	preset, ok := cloudModelPresets[s]
	if !ok {
		return CloudModel{}, fmt.Errorf("unknown cloud model preset %q; use claude-sonnet, gpt-4o-mini, or NAME:in:out", s)
	}
	return preset, nil
}

// energyNote is the portion of the stet finish note used for energy aggregation.
// Usage fields are optional (omitted when STET_CAPTURE_USAGE=false).
type energyNote struct {
	EvalDurationNs   *int64 `json:"eval_duration_ns,omitempty"`
	PromptTokens     *int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens *int64 `json:"completion_tokens,omitempty"`
}

// EnergyResult holds aggregated energy and cost metrics over a ref range.
type EnergyResult struct {
	SessionsCount        int                `json:"sessions_count"`
	TotalEvalDurationNs  int64              `json:"total_eval_duration_ns"`
	TotalPromptTokens    int64              `json:"total_prompt_tokens"`
	TotalCompletionTokens int64             `json:"total_completion_tokens"`
	LocalEnergyKWh       float64            `json:"local_energy_kwh"`
	CloudCostAvoided     map[string]float64 `json:"cloud_cost_avoided"`
}

// Energy walks the ref range since..until, reads stet notes at each commit,
// and returns aggregated energy and cloud cost metrics. watts is the assumed
// power draw (W) for local energy calculation. cloudModels define pricing for
// cloud cost avoided; when empty, only local energy is computed. Malformed
// notes are skipped.
func Energy(repoRoot, sinceRef, untilRef string, watts int, cloudModels []CloudModel) (*EnergyResult, error) {
	shas, err := git.RevList(repoRoot, sinceRef, untilRef)
	if err != nil {
		return nil, err
	}
	res := &EnergyResult{
		CloudCostAvoided: map[string]float64{},
	}
	if len(shas) == 0 {
		return res, nil
	}
	var totalEvalNs int64
	var totalPrompt int64
	var totalCompletion int64
	for _, sha := range shas {
		body, err := git.GetNote(repoRoot, git.NotesRefStet, sha)
		if err != nil {
			continue
		}
		var note energyNote
		if err := json.Unmarshal([]byte(body), &note); err != nil {
			continue
		}
		res.SessionsCount++
		if note.EvalDurationNs != nil {
			totalEvalNs += *note.EvalDurationNs
		}
		if note.PromptTokens != nil {
			totalPrompt += *note.PromptTokens
		}
		if note.CompletionTokens != nil {
			totalCompletion += *note.CompletionTokens
		}
	}
	res.TotalEvalDurationNs = totalEvalNs
	res.TotalPromptTokens = totalPrompt
	res.TotalCompletionTokens = totalCompletion
	// Local energy (kWh): (sum_sec / 3600) * (watts / 1000)
	if totalEvalNs > 0 && watts > 0 {
		sumSec := float64(totalEvalNs) / 1e9
		res.LocalEnergyKWh = (sumSec / 3600) * (float64(watts) / 1000)
	}
	// Cloud cost ($): (prompt/1e6)*in + (completion/1e6)*out per model
	for _, m := range cloudModels {
		cost := (float64(totalPrompt)/1e6)*m.InPerMillion + (float64(totalCompletion)/1e6)*m.OutPerMillion
		res.CloudCostAvoided[m.Name] = cost
	}
	return res, nil
}
