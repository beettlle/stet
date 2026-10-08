// Rules-compliance annotation: for each finding in a hunk, ask System One
// (choice protocol) which applicable project guideline section the finding's
// suggestion most likely conflicts with, and record it on the finding as an
// optional RuleConflict. Human-visible only — this annotator never drops,
// reorders, or rescores findings. Library only; pipeline wiring is a separate
// task (SP-034).
package review

import (
	"context"
	"fmt"
	"strings"

	"stet/cli/internal/findings"
	"stet/cli/internal/rules"
	"stet/cli/internal/systemone"
)

const (
	// annotateMaxQuestionsPerRequest caps how many choice questions a single
	// System One request carries so a large finding batch cannot produce an
	// unbounded payload; the annotator loops over fixed-size chunks.
	annotateMaxQuestionsPerRequest = 32

	// annotateMaxOptions caps how many guideline sections become choice
	// options in one question; sections beyond the cap are not candidates.
	annotateMaxOptions = 32

	// annotateMaxQuestions is the SP-020 question budget (one question per
	// finding): batches larger than this are skipped — not degraded — per the
	// rules-compliance research recommendation.
	annotateMaxQuestions = 64

	// annotateMinProbability is the minimum P(choice) for an annotation to be
	// recorded; lower-confidence verdicts leave RuleConflict nil.
	annotateMinProbability = 0.5
)

// annotateNoneOption is the option ID the decider picks when applying the
// suggestion conflicts with none of the candidate sections.
const annotateNoneOption = "none"

// annotateNoneDescription is the description of the none option.
const annotateNoneDescription = "Applying the suggestion conflicts with none of these guidelines."

// annotateFindingPrefix prefixes the per-question instructions for one finding.
const annotateFindingPrefix = "A code reviewer reported this finding:"

// annotateInstructionsSuffix asks the conflict question shared by every
// annotation request.
const annotateInstructionsSuffix = "\n\nWhich project guideline section listed in the options would applying this suggestion most likely conflict with? Choose the one conflicting section, or none if applying the suggestion conflicts with none of the listed guidelines."

// AnnotateRuleConflicts asks the decision server one choice question per
// finding — which guideline section applying the finding's suggestion would
// conflict with — and returns a new slice of the same length and order with
// Finding.RuleConflict set when the answer names a section with probability at
// least annotateMinProbability. Findings are never dropped, reordered, or
// rescored, and the input slice is not mutated.
//
// At most annotateMaxQuestionsPerRequest questions ride one DecideChoice
// request; at most annotateMaxOptions sections become options (plus none).
// Batches larger than annotateMaxQuestions (the SP-020 question budget) are
// skipped with no request.
//
// nil client, empty model, no sections, empty batch, or an over-budget batch
// returns a copy of batch unchanged. Decider errors and missing answers are
// wrapped with context and returned as (nil, err); the caller decides whether
// to warn and continue.
//
// Security: hunk content is attacker-influenced input (e.g. diff comments), so
// it is passed only in the request state and never echoed into question
// instructions or option descriptions. Instructions carry the finding's
// message and suggestion, which are review-model output rather than raw diff,
// but a crafted diff could still bias the verdict.
func AnnotateRuleConflicts(ctx context.Context, client rules.ChoiceDecider, model string, batch []findings.Finding, hunkContent string, sections []rules.Section) ([]findings.Finding, error) {
	if client == nil || model == "" || len(sections) == 0 || len(batch) == 0 || len(batch) > annotateMaxQuestions {
		return copyFindingBatch(batch), nil
	}

	capped := sections
	if len(capped) > annotateMaxOptions {
		capped = capped[:annotateMaxOptions]
	}
	options := make(map[string]string, len(capped)+1)
	sectionByKey := make(map[string]rules.Section, len(capped))
	for i, s := range capped {
		key := annotateOptionKey(i)
		options[key] = s.Source + ": " + s.Heading
		sectionByKey[key] = s
	}
	options[annotateNoneOption] = annotateNoneDescription

	state := buildSystemOneState(hunkContent)
	conflicts := make([]*findings.RuleConflict, len(batch))
	for start := 0; start < len(batch); start += annotateMaxQuestionsPerRequest {
		end := start + annotateMaxQuestionsPerRequest
		if end > len(batch) {
			end = len(batch)
		}
		req := systemone.ChoiceRequest{
			Model:     model,
			State:     state,
			Questions: make(map[string]systemone.ChoiceQuestion, end-start),
		}
		for i := start; i < end; i++ {
			req.Questions[annotateQuestionKey(i)] = systemone.ChoiceQuestion{
				Instructions: buildAnnotateInstructions(batch[i]),
				Criteria:     options,
			}
		}
		resp, err := client.DecideChoice(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("rules annotate: request for findings %d-%d: %w", start, end-1, err)
		}
		for i := start; i < end; i++ {
			answer, ok := resp.Answers[annotateQuestionKey(i)]
			if !ok {
				return nil, fmt.Errorf("rules annotate: response missing answer for question %q", annotateQuestionKey(i))
			}
			// none, unknown option IDs, and sub-threshold probabilities all
			// leave the finding unannotated; nothing is ever dropped.
			section, ok := sectionByKey[answer.Choice]
			if !ok {
				continue
			}
			prob := answer.Probabilities[answer.Choice]
			if prob < annotateMinProbability {
				continue
			}
			conflicts[i] = &findings.RuleConflict{Source: section.Source, Section: section.Heading, Probability: prob}
		}
	}

	out := make([]findings.Finding, len(batch))
	copy(out, batch)
	for i, c := range conflicts {
		if c != nil {
			out[i].RuleConflict = c
		}
	}
	return out, nil
}

// annotateQuestionKey returns the question key for the finding at batch index
// i ("q0", "q1", ...). Keys use the batch index, never the finding ID, so
// duplicate finding IDs cannot collide within a request.
func annotateQuestionKey(i int) string {
	return fmt.Sprintf("q%d", i)
}

// annotateOptionKey returns the option key for the section at index i
// ("s0", "s1", ...).
func annotateOptionKey(i int) string {
	return fmt.Sprintf("s%d", i)
}

// copyFindingBatch returns a shallow copy of batch with the same length and
// order; mutating the copy never mutates the input slice.
func copyFindingBatch(batch []findings.Finding) []findings.Finding {
	out := make([]findings.Finding, len(batch))
	copy(out, batch)
	return out
}

// buildAnnotateInstructions formats one finding's message and suggestion. Only
// finding metadata appears here; the code under review stays in the request
// state and is never echoed into instructions.
func buildAnnotateInstructions(f findings.Finding) string {
	var b strings.Builder
	b.WriteString(annotateFindingPrefix)
	b.WriteString("\n- message: ")
	b.WriteString(f.Message)
	if f.Suggestion != "" {
		b.WriteString("\n- suggestion: ")
		b.WriteString(f.Suggestion)
	}
	b.WriteString(annotateInstructionsSuffix)
	return b.String()
}
