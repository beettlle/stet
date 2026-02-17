// Package stats provides impact reporting (volume, quality, energy) from
// stet git notes and history.

package stats

import (
	"os"

	"stet/cli/internal/history"
)

// QualityResult holds aggregated quality metrics from .review/history.jsonl.
type QualityResult struct {
	SessionsCount      int                `json:"sessions_count"`
	TotalFindings      int                `json:"total_findings"`
	TotalDismissed     int                `json:"total_dismissed"`
	DismissalRate      float64            `json:"dismissal_rate"`
	AcceptanceRate     float64            `json:"acceptance_rate"`
	FalsePositiveRate  float64            `json:"false_positive_rate"`
	Actionability     float64            `json:"actionability"`
	CleanCommitRate    float64            `json:"clean_commit_rate"`
	FindingDensity    float64            `json:"finding_density,omitempty"` // Omitted when no tokens.
	DismissalsByReason map[string]int     `json:"dismissals_by_reason"`
	CategoryBreakdown  map[string]int     `json:"category_breakdown"`
}

// Quality reads .review/history.jsonl from stateDir (including rotated archives),
// aggregates findings and dismissals, and returns quality metrics. When stateDir
// does not exist or history is empty, returns a result with zero counts and no error.
func Quality(stateDir string) (*QualityResult, error) {
	if _, err := os.Stat(stateDir); err != nil && os.IsNotExist(err) {
		return &QualityResult{
			DismissalsByReason: map[string]int{},
			CategoryBreakdown:  map[string]int{},
		}, nil
	}
	records, err := history.ReadRecords(stateDir)
	if err != nil {
		return nil, err
	}
	res := &QualityResult{
		DismissalsByReason: map[string]int{},
		CategoryBreakdown:  map[string]int{},
	}
	var sessionsWithZeroFindings int
	var tokensReviewed int64
	for _, rec := range records {
		res.SessionsCount++
		nFindings := len(rec.ReviewOutput)
		res.TotalFindings += nFindings
		if nFindings == 0 {
			sessionsWithZeroFindings++
		}
		res.TotalDismissed += len(rec.UserAction.DismissedIDs)
		for _, d := range rec.UserAction.Dismissals {
			if d.Reason != "" {
				res.DismissalsByReason[d.Reason]++
			}
		}
		for _, f := range rec.ReviewOutput {
			res.CategoryBreakdown[string(f.Category)]++
		}
		// Tokens: prefer Record-level; fall back to Usage.
		var prompt, completion int64
		if rec.PromptTokens != nil {
			prompt = *rec.PromptTokens
		} else if rec.Usage != nil && rec.Usage.PromptTokens != nil {
			prompt = *rec.Usage.PromptTokens
		}
		if rec.CompletionTokens != nil {
			completion = *rec.CompletionTokens
		} else if rec.Usage != nil && rec.Usage.CompletionTokens != nil {
			completion = *rec.Usage.CompletionTokens
		}
		tokensReviewed += prompt + completion
	}
	// Rates with safe division.
	if res.TotalFindings > 0 {
		res.DismissalRate = float64(res.TotalDismissed) / float64(res.TotalFindings)
		res.AcceptanceRate = float64(res.TotalFindings-res.TotalDismissed) / float64(res.TotalFindings)
		res.FalsePositiveRate = float64(res.DismissalsByReason[history.ReasonFalsePositive]) / float64(res.TotalFindings)
	}
	if res.TotalDismissed > 0 {
		res.Actionability = float64(res.DismissalsByReason[history.ReasonAlreadyCorrect]) / float64(res.TotalDismissed)
	}
	if res.SessionsCount > 0 {
		res.CleanCommitRate = float64(sessionsWithZeroFindings) / float64(res.SessionsCount)
	}
	if tokensReviewed > 0 && res.TotalFindings > 0 {
		res.FindingDensity = float64(res.TotalFindings) / (float64(tokensReviewed) / 1000)
	}
	return res, nil
}
