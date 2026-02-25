// Package stats (volume.go) aggregates review volume metrics (hunks, lines,
// chars) from stet git notes across a ref range.
package stats

import (
	"encoding/json"

	"stet/cli/internal/git"
)

// StetNote is the scope portion of the note written on stet finish.
// Used to parse refs/notes/stet for volume aggregation.
type StetNote struct {
	HunksReviewed  int `json:"hunks_reviewed"`
	LinesAdded     int `json:"lines_added"`
	LinesRemoved   int `json:"lines_removed"`
	CharsAdded     int `json:"chars_added"`
	CharsDeleted   int `json:"chars_deleted"`
	CharsReviewed  int `json:"chars_reviewed"`
}

// VolumeResult holds aggregated volume metrics over a ref range.
// GitAI is populated only when git-ai is detected in the repo (refs/notes/ai exists).
type VolumeResult struct {
	SessionsCount          int          `json:"sessions_count"`
	CommitsInRange         int          `json:"commits_in_range"`
	CommitsWithNote        int          `json:"commits_with_note"`
	TotalHunksReviewed     int          `json:"total_hunks_reviewed"`
	TotalLinesAdded        int          `json:"total_lines_added"`
	TotalLinesRemoved      int          `json:"total_lines_removed"`
	TotalCharsAdded        int          `json:"total_chars_added"`
	TotalCharsDeleted      int          `json:"total_chars_deleted"`
	TotalCharsReviewed     int          `json:"total_chars_reviewed"`
	PercentCommitsWithNote float64      `json:"percent_commits_with_note"`
	GitAI                  *GitAIResult `json:"git_ai,omitempty"`
}

// Volume walks the ref range since..until, reads stet notes at each commit,
// and returns aggregated volume metrics. Malformed notes are skipped.
func Volume(repoRoot, sinceRef, untilRef string) (*VolumeResult, error) {
	shas, err := git.RevList(repoRoot, sinceRef, untilRef)
	if err != nil {
		return nil, err
	}
	res := &VolumeResult{
		CommitsInRange: len(shas),
	}
	if len(shas) == 0 {
		return res, nil
	}
	for _, sha := range shas {
		body, err := git.GetNote(repoRoot, git.NotesRefStet, sha)
		if err != nil {
			continue
		}
		var note StetNote
		if err := json.Unmarshal([]byte(body), &note); err != nil {
			continue
		}
		res.CommitsWithNote++
		res.SessionsCount++
		res.TotalHunksReviewed += note.HunksReviewed
		res.TotalLinesAdded += note.LinesAdded
		res.TotalLinesRemoved += note.LinesRemoved
		res.TotalCharsAdded += note.CharsAdded
		res.TotalCharsDeleted += note.CharsDeleted
		res.TotalCharsReviewed += note.CharsReviewed
	}
	if res.CommitsInRange > 0 {
		res.PercentCommitsWithNote = 100 * float64(res.CommitsWithNote) / float64(res.CommitsInRange)
	}
	if inUse, err := GitAIInUse(repoRoot); err == nil && inUse {
		gitAI, err := GitAIMetrics(repoRoot, sinceRef, untilRef)
		if err == nil {
			res.GitAI = gitAI
		}
	}
	return res, nil
}
