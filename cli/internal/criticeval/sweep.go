// Sweep scores a labeled dataset with the System One critic and evaluates
// candidate thresholds: for each threshold it reports how many false positives
// would be caught and how many real findings would be lost, then recommends
// the highest threshold that keeps keeps-lost within budget. This is the
// computation behind `stet critic-eval`; it sends no requests itself.
package criticeval

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"stet/cli/internal/findings"
	"stet/cli/internal/review"
)

// Threshold sweep and recommendation constants.
const (
	// The sweep evaluates thresholds step/sweepThresholdDivisor for step in
	// [sweepMinStep, sweepMaxStep] — 0.1, 0.2, …, 0.9. Division (not repeated
	// addition) keeps each value equal to its decimal literal exactly.
	sweepMinStep          = 1
	sweepMaxStep          = 9
	sweepThresholdDivisor = 10.0

	// maxKeepsLostPct is the largest share of keep examples (in percent) a
	// recommended threshold may lose.
	maxKeepsLostPct = 10.0

	// minReliableExamples is the minimum number of examples per label for the
	// recommendation to be considered reliable.
	minReliableExamples = 10

	// warningUnreliable is reported when either label has fewer than
	// minReliableExamples examples.
	warningUnreliable = "recommendation unreliable"
)

// ThresholdResult summarizes one candidate threshold over the labeled scores.
type ThresholdResult struct {
	Threshold      float64
	DropsCaughtPct float64 // share of "drop" examples with score < threshold
	KeepsLostPct   float64 // share of "keep" examples with score < threshold
	Kept           int     // examples (either label) with score >= threshold
}

// Report is the outcome of one threshold sweep over a scored dataset.
type Report struct {
	Counts                 map[Label]int
	Thresholds             []ThresholdResult
	LatencyP50, LatencyP95 time.Duration
	Recommended            *float64 // nil when no threshold qualifies
	Warnings               []string
}

// Score verifies every example in ds with the System One critic, sending one
// VerifyFindingsSystemOne call per hunk group (minKeep 0, so every P(yes)
// score comes back) and returns finding ID -> P(yes) plus one wall-time
// duration per group call. Missing finding IDs produce no score; Evaluate
// treats those examples as scored 0. Errors abort with the hunk group index.
func Score(ctx context.Context, client review.SystemOneDecider, model string, samples int, ds *Dataset) (map[string]float64, []time.Duration, error) {
	if ds == nil {
		return nil, nil, errors.New("critic eval: nil dataset")
	}
	if client == nil {
		return nil, nil, errors.New("critic eval: nil System One client")
	}
	if model == "" {
		return nil, nil, errors.New("critic eval: empty model")
	}
	scores := make(map[string]float64)
	latencies := make([]time.Duration, 0, len(ds.Groups))
	for i, group := range ds.Groups {
		batch := make([]findings.Finding, len(group.Examples))
		for j, ex := range group.Examples {
			batch[j] = ex.Finding
		}
		start := time.Now()
		_, groupScores, err := review.VerifyFindingsSystemOne(ctx, client, model, batch, group.Hunk, 0, samples)
		elapsed := time.Since(start)
		if err != nil {
			return nil, nil, fmt.Errorf("critic eval: hunk group %d: %w", i, err)
		}
		latencies = append(latencies, elapsed)
		for id, p := range groupScores {
			scores[id] = p
		}
	}
	return scores, latencies, nil
}

// Evaluate sweeps candidate thresholds over scored examples and returns the
// resulting Report. Scores are looked up by finding ID; an example without a
// score counts as 0, i.e. it would be dropped at every threshold. Latency
// percentiles use the nearest-rank method on the sorted durations (empty
// input yields zero). The recommendation is the highest threshold whose
// KeepsLostPct is at most maxKeepsLostPct, or nil when none qualifies. A
// warning is added when either label has fewer than minReliableExamples
// examples. A nil dataset evaluates as empty.
func Evaluate(scores map[string]float64, latencies []time.Duration, ds *Dataset) Report {
	rep := Report{Counts: map[Label]int{LabelKeep: 0, LabelDrop: 0}}
	var keeps, drops []float64
	if ds != nil {
		rep.Counts[LabelKeep] = ds.Counts[LabelKeep]
		rep.Counts[LabelDrop] = ds.Counts[LabelDrop]
		for _, group := range ds.Groups {
			for _, ex := range group.Examples {
				s := 0.0 // No score: treated as dropped at every threshold.
				if v, ok := scores[ex.Finding.ID]; ok {
					s = v
				}
				switch ex.Label {
				case LabelKeep:
					keeps = append(keeps, s)
				case LabelDrop:
					drops = append(drops, s)
				}
			}
		}
	}
	total := len(keeps) + len(drops)
	for step := sweepMinStep; step <= sweepMaxStep; step++ {
		t := float64(step) / sweepThresholdDivisor
		caught, lost := 0, 0
		for _, s := range drops {
			if s < t {
				caught++
			}
		}
		for _, s := range keeps {
			if s < t {
				lost++
			}
		}
		res := ThresholdResult{
			Threshold:      t,
			DropsCaughtPct: pct(caught, len(drops)),
			KeepsLostPct:   pct(lost, len(keeps)),
			Kept:           total - caught - lost,
		}
		rep.Thresholds = append(rep.Thresholds, res)
		if res.KeepsLostPct <= maxKeepsLostPct {
			recommended := t
			rep.Recommended = &recommended
		}
	}
	if rep.Counts[LabelKeep] < minReliableExamples || rep.Counts[LabelDrop] < minReliableExamples {
		rep.Warnings = append(rep.Warnings, warningUnreliable)
	}
	rep.LatencyP50 = percentile(latencies, 50)
	rep.LatencyP95 = percentile(latencies, 95)
	return rep
}

// pct returns part/total as a percentage; 0 when total is 0.
func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part*100) / float64(total)
}

// percentile returns the nearest-rank p-th percentile of latencies: rank
// ceil(p/100 * N) over the sorted durations, clamped to [1, N]. Empty input
// returns 0.
func percentile(latencies []time.Duration, p int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	sorted := slices.Clone(latencies)
	slices.Sort(sorted)
	rank := (p*len(sorted) + 99) / 100
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}
