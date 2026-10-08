// Rules-compliance labeled pairs: a reusable (premise, snippet, label)
// dataset with a JSON Lines loader and an evaluator that scores pairs with
// the System One decider using the SP-020 Protocol A noul protocol
// (docs/research/systemone-rules-compliance.md, Q3): one request per
// identical snippet, one question per pair, P(yes) >= threshold predicts
// "violating". Unlike dataset.go/sweep.go this dataset is fully curated
// (committed JSONL), not derived from review state.
package criticeval

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"stet/cli/internal/review"
	"stet/cli/internal/systemone"
)

// RulePairLabel is the ground-truth label for one (premise, snippet) pair.
type RulePairLabel string

// Rule pair labels.
const (
	RulePairViolating RulePairLabel = "violating"
	RulePairCompliant RulePairLabel = "compliant"
)

// rulePairMaxQuestionsPerRequest caps how many rule-pair questions a single
// System One request carries (same chunk size as the finding verifier).
const rulePairMaxQuestionsPerRequest = 32

// Rule pair noul criteria text, fixed for every pair (SP-020 Protocol A).
const (
	rulePairCriteriaTrue  = "The code violates the stated project rule."
	rulePairCriteriaFalse = "The code complies with the rule or the rule does not apply to this code."
)

// RulePair is one labeled (premise, snippet) pair for rules-compliance
// evaluation. Premise is the atomic rule statement judged against Snippet;
// Source names the rule area the premise paraphrases; Origin records where
// the pair came from ("sp020-seed" or "synthetic").
type RulePair struct {
	ID        string        `json:"id"`
	Language  string        `json:"language"`
	PremiseID string        `json:"premise_id"`
	Premise   string        `json:"premise"`
	Source    string        `json:"source"`
	Snippet   string        `json:"snippet"`
	Label     RulePairLabel `json:"label"`
	Origin    string        `json:"origin"`
}

// rulePairRequiredFields lists the string fields every record must set, in
// declaration order, so validation errors are deterministic.
var rulePairRequiredFields = []struct {
	name  string
	value func(p RulePair) string
}{
	{"id", func(p RulePair) string { return p.ID }},
	{"language", func(p RulePair) string { return p.Language }},
	{"premise_id", func(p RulePair) string { return p.PremiseID }},
	{"premise", func(p RulePair) string { return p.Premise }},
	{"source", func(p RulePair) string { return p.Source }},
	{"snippet", func(p RulePair) string { return p.Snippet }},
	{"origin", func(p RulePair) string { return p.Origin }},
}

// LoadRulePairs reads a JSON Lines file of rule pairs. Every non-empty line
// must be a JSON object with a known label and all required fields non-empty;
// IDs must be unique across the file. Errors name the 1-based line number of
// the offending record.
func LoadRulePairs(path string) ([]RulePair, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("rule pairs: open %s: %w", path, err)
	}
	defer f.Close()

	var pairs []RulePair
	seen := make(map[string]struct{})
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var p RulePair
		if err := json.Unmarshal([]byte(text), &p); err != nil {
			return nil, fmt.Errorf("rule pairs: line %d: invalid JSON: %w", line, err)
		}
		switch p.Label {
		case RulePairViolating, RulePairCompliant:
		default:
			return nil, fmt.Errorf("rule pairs: line %d: unknown label %q", line, p.Label)
		}
		for _, field := range rulePairRequiredFields {
			if field.value(p) == "" {
				return nil, fmt.Errorf("rule pairs: line %d: empty field %q", line, field.name)
			}
		}
		if _, dup := seen[p.ID]; dup {
			return nil, fmt.Errorf("rule pairs: line %d: duplicate id %q", line, p.ID)
		}
		seen[p.ID] = struct{}{}
		pairs = append(pairs, p)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("rule pairs: read %s: %w", path, err)
	}
	return pairs, nil
}

// RulePairReport summarizes one rules-compliance evaluation run.
type RulePairReport struct {
	Total, Violating, Compliant int
	Accuracy                    float64            // share judged correctly at the threshold
	AUROC                       float64            // rank-based Mann-Whitney; 0 when a class is empty
	ByLanguage                  map[string]float64 // accuracy per language
	Misses                      []string           // misclassified pair IDs, in input order
}

// EvaluateRulePairs scores every pair against the System One decider and
// returns the resulting report. The premise text goes in the question
// instructions, the snippet only in the request state; pairs sharing an
// identical snippet share requests, at most rulePairMaxQuestionsPerRequest
// questions per request. P(yes) >= threshold predicts "violating". An empty
// input returns an empty report without sending any request. Errors from the
// decider are wrapped with the ID of the pair being evaluated.
func EvaluateRulePairs(ctx context.Context, client review.SystemOneDecider, model string, pairs []RulePair, threshold float64) (RulePairReport, error) {
	if client == nil {
		return RulePairReport{}, errors.New("rule pairs: nil System One client")
	}
	if model == "" {
		return RulePairReport{}, errors.New("rule pairs: empty model")
	}
	scores, err := scoreRulePairs(ctx, client, model, pairs)
	if err != nil {
		return RulePairReport{}, err
	}
	return buildRulePairReport(pairs, scores, threshold), nil
}

// scoreRulePairs requests a P(yes) score for every pair, batching pairs that
// share an identical snippet into shared requests (chunked at
// rulePairMaxQuestionsPerRequest questions). Snippet groups are requested in
// first-appearance order so the request sequence is deterministic.
func scoreRulePairs(ctx context.Context, client review.SystemOneDecider, model string, pairs []RulePair) (map[string]float64, error) {
	scores := make(map[string]float64, len(pairs))
	order := make([]string, 0, len(pairs))
	bySnippet := make(map[string][]int, len(pairs))
	for i, p := range pairs {
		if _, ok := bySnippet[p.Snippet]; !ok {
			order = append(order, p.Snippet)
		}
		bySnippet[p.Snippet] = append(bySnippet[p.Snippet], i)
	}
	for _, snippet := range order {
		indexes := bySnippet[snippet]
		for start := 0; start < len(indexes); start += rulePairMaxQuestionsPerRequest {
			end := start + rulePairMaxQuestionsPerRequest
			if end > len(indexes) {
				end = len(indexes)
			}
			if err := decideRuleChunk(ctx, client, model, pairs, indexes[start:end], snippet, scores); err != nil {
				return nil, err
			}
		}
	}
	return scores, nil
}

// decideRuleChunk sends one request whose state is snippet and whose
// questions judge the premise of each pair in chunk, recording each answer's
// P(yes) in scores by pair ID. Errors are wrapped with the pair ID.
func decideRuleChunk(ctx context.Context, client review.SystemOneDecider, model string, pairs []RulePair, chunk []int, snippet string, scores map[string]float64) error {
	req := systemone.Request{
		Model:     model,
		State:     snippet,
		Questions: make(map[string]systemone.NoulQuestion, len(chunk)),
	}
	for i, idx := range chunk {
		req.Questions[rulePairQuestionKey(i)] = systemone.NoulQuestion{
			Instructions: rulePairInstructions(pairs[idx].Premise),
			Criteria: systemone.NoulCriteria{
				True:  rulePairCriteriaTrue,
				False: rulePairCriteriaFalse,
			},
		}
	}
	resp, err := client.Decide(ctx, req)
	if err != nil {
		return fmt.Errorf("rule pairs: evaluate pair %s: %w", pairs[chunk[0]].ID, err)
	}
	for i, idx := range chunk {
		key := rulePairQuestionKey(i)
		answer, ok := resp.Answers[key]
		if !ok {
			return fmt.Errorf("rule pairs: evaluate pair %s: response missing answer for question %q", pairs[idx].ID, key)
		}
		scores[pairs[idx].ID] = answer.Noul
	}
	return nil
}

// rulePairQuestionKey returns the question key for the chunk-local pair index
// i ("q0", "q1", ...). Keys are chunk-local indexes, never pair IDs, so
// duplicate or odd IDs cannot collide within a request.
func rulePairQuestionKey(i int) string {
	return fmt.Sprintf("q%d", i)
}

// rulePairInstructions formats one pair's question instructions: the premise
// text plus the violation question. The snippet is never echoed here; it
// travels only in the request state.
func rulePairInstructions(premise string) string {
	return "Project rule: " + premise + "\n\nDoes this code violate the rule?"
}

// buildRulePairReport assembles the report from per-pair P(yes) scores.
// A pair with score >= threshold is predicted violating. ByLanguage holds
// accuracy per language present in pairs; Misses keeps input order.
func buildRulePairReport(pairs []RulePair, scores map[string]float64, threshold float64) RulePairReport {
	rep := RulePairReport{Total: len(pairs), ByLanguage: make(map[string]float64)}
	var (
		correct              int
		violating, compliant []float64
		langTotal            = make(map[string]int)
		langOK               = make(map[string]int)
	)
	for _, p := range pairs {
		score := scores[p.ID]
		switch p.Label {
		case RulePairViolating:
			rep.Violating++
			violating = append(violating, score)
		case RulePairCompliant:
			rep.Compliant++
			compliant = append(compliant, score)
		}
		predicted := score >= threshold
		if predicted == (p.Label == RulePairViolating) {
			correct++
			langOK[p.Language]++
		} else {
			rep.Misses = append(rep.Misses, p.ID)
		}
		langTotal[p.Language]++
	}
	if rep.Total > 0 {
		rep.Accuracy = float64(correct) / float64(rep.Total)
	}
	for lang, total := range langTotal {
		rep.ByLanguage[lang] = float64(langOK[lang]) / float64(total)
	}
	rep.AUROC = rulePairAUROC(violating, compliant)
	return rep
}

// rulePairAUROC computes the rank-based (Mann-Whitney U) AUROC: the
// probability that a random violating pair scores above a random compliant
// pair, with ties counting half. It returns 0 (never NaN) when either class
// is empty.
func rulePairAUROC(positives, negatives []float64) float64 {
	np, nn := len(positives), len(negatives)
	if np == 0 || nn == 0 {
		return 0
	}
	sorted := make([]float64, 0, np+nn)
	sorted = append(sorted, positives...)
	sorted = append(sorted, negatives...)
	slices.Sort(sorted)
	var rankSum float64
	for _, v := range positives {
		rankSum += rulePairAverageRank(sorted, v)
	}
	u := rankSum - float64(np*(np+1))/2
	return u / (float64(np) * float64(nn))
}

// rulePairAverageRank returns the average 1-based rank of value v in the
// sorted slice, averaging tied positions (fractional ranks).
func rulePairAverageRank(sorted []float64, v float64) float64 {
	first := sort.SearchFloat64s(sorted, v)                                     // first index with sorted[i] >= v
	last := sort.Search(len(sorted), func(i int) bool { return sorted[i] > v }) // one past the last equal index
	return float64(first+1+last) / 2
}
