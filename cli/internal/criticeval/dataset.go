// Package criticeval builds the labeled dataset used to choose the System One
// critic threshold (systemone_min_keep). Labels come from local review state:
// dismissals with reason false_positive or wrong_suggestion (and a stored
// prompt context) are negatives; undismissed active session findings with a
// stored prompt context are weak positives. A curated JSONL file can replace
// local state entirely. Examples are grouped by exact hunk text so a later
// evaluation can verify each group with one System One request.
// The loader is strictly read-only over .review state.
package criticeval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"stet/cli/internal/erruser"
	"stet/cli/internal/findings"
	"stet/cli/internal/history"
	"stet/cli/internal/session"
)

// Label is the evaluation label for one finding: keep = real finding
// (positive), drop = false positive (negative).
type Label string

// Evaluation labels.
const (
	LabelKeep Label = "keep"
	LabelDrop Label = "drop"
)

// Example source values (Dataset.BySource keys and Example.Source).
// SourceConflict counts finding IDs with contradictory labels; such
// examples are removed from the dataset.
const (
	SourceHistory  = "history"
	SourceSession  = "session"
	SourceCurated  = "curated"
	SourceConflict = "conflict"
)

// Example is one labeled finding with the code context it was raised on.
type Example struct {
	Finding findings.Finding
	Label   Label
	Source  string
}

// HunkGroup groups examples that share the same hunk text so one System One
// request can cover the whole group.
type HunkGroup struct {
	Hunk     string
	Examples []Example
}

// Dataset is a grouped, de-duplicated set of labeled examples.
// Counts has one entry per label (zero when absent); BySource counts examples
// per source, plus SourceConflict for removed contradictory finding IDs.
type Dataset struct {
	Groups   []HunkGroup
	Counts   map[Label]int
	BySource map[string]int
}

// LoadFromState reads .review state from stateDir and returns labeled
// examples grouped by hunk. Drops come from history dismissals with reason
// false_positive or wrong_suggestion and a non-empty prompt_context, resolved
// against the same record's review_output by finding ID. Keeps are weak
// labels: active session findings (not in dismissed_ids) with an entry in
// finding_prompt_context. already_correct and out_of_scope dismissals are
// excluded as ambiguous. A missing stateDir yields an empty dataset.
// LoadFromState never writes to stateDir.
func LoadFromState(stateDir string) (*Dataset, error) {
	c := newCollector()
	if _, err := os.Stat(stateDir); err != nil {
		if os.IsNotExist(err) {
			return c.dataset(), nil
		}
		return nil, erruser.New("Could not read review state directory.", err)
	}
	records, err := history.ReadRecords(stateDir)
	if err != nil {
		return nil, err
	}
	for _, rec := range records {
		byID := make(map[string]findings.Finding, len(rec.ReviewOutput))
		for _, f := range rec.ReviewOutput {
			if f.ID != "" {
				byID[f.ID] = f
			}
		}
		for _, d := range rec.UserAction.Dismissals {
			switch d.Reason {
			case history.ReasonFalsePositive, history.ReasonWrongSuggestion:
			default:
				continue // already_correct / out_of_scope / empty: excluded.
			}
			if d.FindingID == "" || d.PromptContext == "" {
				continue
			}
			f, ok := byID[d.FindingID]
			if !ok {
				continue // Cannot resolve dismissal to a finding.
			}
			c.add(d.FindingID, Example{Finding: f, Label: LabelDrop, Source: SourceHistory}, d.PromptContext)
		}
	}
	s, err := session.Load(stateDir)
	if err != nil {
		return nil, err
	}
	dismissed := make(map[string]struct{}, len(s.DismissedIDs))
	for _, id := range s.DismissedIDs {
		dismissed[id] = struct{}{}
	}
	for _, f := range s.Findings {
		if f.ID == "" {
			continue
		}
		if _, ok := dismissed[f.ID]; ok {
			continue // Only active findings are weak positives.
		}
		hunk, ok := s.FindingPromptContext[f.ID]
		if !ok || hunk == "" {
			continue
		}
		c.add(f.ID, Example{Finding: f, Label: LabelKeep, Source: SourceSession}, hunk)
	}
	return c.dataset(), nil
}

// curatedLine is one JSONL line of a curated dataset file.
type curatedLine struct {
	Finding findings.Finding `json:"finding"`
	Hunk    string           `json:"hunk"`
	Label   Label            `json:"label"`
}

// LoadDataset reads a curated JSONL dataset where each non-empty line is
// {"finding": {...}, "hunk": "...", "label": "keep"|"drop"}. It replaces
// history/session sources entirely. Unknown labels and empty hunks are
// errors naming the line number. Lines without a finding ID get a synthetic
// per-line key so grouping and conflict handling still work.
func LoadDataset(path string) (*Dataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, erruser.New("Could not open curated dataset file.", err)
	}
	defer func() { _ = f.Close() }()
	c := newCollector()
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var cl curatedLine
		if err := json.Unmarshal([]byte(line), &cl); err != nil {
			return nil, fmt.Errorf("curated dataset line %d: invalid JSON: %w", lineNo, err)
		}
		if cl.Label != LabelKeep && cl.Label != LabelDrop {
			return nil, fmt.Errorf("curated dataset line %d: unknown label %q (want %q or %q)", lineNo, cl.Label, LabelKeep, LabelDrop)
		}
		if cl.Hunk == "" {
			return nil, fmt.Errorf("curated dataset line %d: hunk is empty", lineNo)
		}
		id := cl.Finding.ID
		if id == "" {
			id = fmt.Sprintf("line:%d", lineNo)
		}
		c.add(id, Example{Finding: cl.Finding, Label: cl.Label, Source: SourceCurated}, cl.Hunk)
	}
	if err := sc.Err(); err != nil {
		return nil, erruser.New("Could not read curated dataset file.", err)
	}
	return c.dataset(), nil
}

// entry pairs an example with the hunk text used for grouping.
type entry struct {
	ex   Example
	hunk string
}

// collector accumulates examples by finding ID with de-duplication and
// conflict removal. First occurrence wins; an ID seen with both labels is
// removed entirely and counted as a conflict.
type collector struct {
	byID     map[string]entry
	conflict map[string]struct{}
	order    []string // Finding IDs in first-seen order (deterministic output).
}

func newCollector() *collector {
	return &collector{
		byID:     make(map[string]entry),
		conflict: make(map[string]struct{}),
	}
}

func (c *collector) add(id string, ex Example, hunk string) {
	if _, bad := c.conflict[id]; bad {
		return // Already removed as contradictory; stay removed.
	}
	prev, ok := c.byID[id]
	if ok {
		if prev.ex.Label != ex.Label {
			delete(c.byID, id)
			c.conflict[id] = struct{}{}
		}
		return // Same label: first occurrence wins.
	}
	c.byID[id] = entry{ex: ex, hunk: hunk}
	c.order = append(c.order, id)
}

// dataset groups accepted examples by exact hunk text and computes counts.
func (c *collector) dataset() *Dataset {
	d := &Dataset{
		Counts:   map[Label]int{LabelKeep: 0, LabelDrop: 0},
		BySource: map[string]int{},
	}
	index := make(map[string]int) // hunk text -> group index in d.Groups.
	for _, id := range c.order {
		e, ok := c.byID[id]
		if !ok {
			continue // Removed as a conflict.
		}
		d.Counts[e.ex.Label]++
		d.BySource[e.ex.Source]++
		gi, ok := index[e.hunk]
		if !ok {
			d.Groups = append(d.Groups, HunkGroup{Hunk: e.hunk})
			gi = len(d.Groups) - 1
			index[e.hunk] = gi
		}
		d.Groups[gi].Examples = append(d.Groups[gi].Examples, e.ex)
	}
	d.BySource[SourceConflict] = len(c.conflict)
	return d
}
