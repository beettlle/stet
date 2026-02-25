// Package stats (gitai.go) provides git-ai integration for stet stats.
// Parses refs/notes/ai per Git AI Standard v3.0.0. Only used when git-ai
// is detected in the repo (refs/notes/ai exists).
package stats

import (
	"encoding/json"
	"strings"

	"stet/cli/internal/git"
)

// GitAIResult holds aggregated git-ai metrics over a ref range.
type GitAIResult struct {
	CommitsWithAINote   int `json:"commits_with_ai_note"`
	TotalAIAuthoredLines int `json:"total_ai_authored_lines"`
}

// gitAIMetadata is the metadata section of a git-ai note (Git AI Standard v3.0.0).
type gitAIMetadata struct {
	SchemaVersion string                    `json:"schema_version"`
	Prompts       map[string]gitAIPromptRec `json:"prompts"`
}

// gitAIPromptRec is a prompt record in the metadata.
// The JSON tag uses the corrected spelling "overridden_lines"; the v3.0.0 spec
// had a typo "overriden_lines" (errata E-001). Old notes with the typo will
// leave OverriddenLines at zero, which is acceptable since the field is
// informational and not used in aggregation logic.
type gitAIPromptRec struct {
	AcceptedLines   int `json:"accepted_lines"`
	TotalAdditions  int `json:"total_additions"`
	TotalDeletions  int `json:"total_deletions"`
	OverriddenLines int `json:"overridden_lines"`
}

// GitAIInUse reports whether git-ai is used in the repo (refs/notes/ai exists).
func GitAIInUse(repoRoot string) (bool, error) {
	return git.RefExists(repoRoot, git.NotesRefAI)
}

// GitAIMetrics walks the ref range, reads git-ai notes at each commit, and
// returns aggregated AI-authored line metrics. Malformed notes are skipped.
// Call only when GitAIInUse returns true.
func GitAIMetrics(repoRoot, sinceRef, untilRef string) (*GitAIResult, error) {
	shas, err := git.RevList(repoRoot, sinceRef, untilRef)
	if err != nil {
		return nil, err
	}
	res := &GitAIResult{}
	if len(shas) == 0 {
		return res, nil
	}
	for _, sha := range shas {
		body, err := git.GetNote(repoRoot, git.NotesRefAI, sha)
		if err != nil {
			continue
		}
		lines, err := parseGitAINote(body)
		if err != nil {
			continue
		}
		res.CommitsWithAINote++
		res.TotalAIAuthoredLines += lines
	}
	return res, nil
}

// parseGitAINote extracts total AI-authored lines from a git-ai note body.
// Format: attestation section, divider "---", metadata JSON. Sums accepted_lines
// across all prompts in the metadata. Returns error for invalid format so
// callers skip the commit (do not count it as having a git-ai note).
func parseGitAINote(body string) (int, error) {
	parts := strings.SplitN(body, "\n---\n", 2)
	if len(parts) != 2 {
		return 0, errInvalidGitAINote
	}
	metaJSON := strings.TrimSpace(parts[1])
	if metaJSON == "" {
		return 0, errInvalidGitAINote
	}
	var meta gitAIMetadata
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		return 0, err
	}
	if meta.Prompts == nil {
		return 0, nil
	}
	var total int
	for _, p := range meta.Prompts {
		total += p.AcceptedLines
	}
	return total, nil
}

var errInvalidGitAINote = &invalidGitAINoteError{}

type invalidGitAINoteError struct{}

func (e *invalidGitAINoteError) Error() string {
	return "invalid git-ai note format"
}
