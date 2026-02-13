package review

import (
	"context"
	"fmt"

	"stet/cli/internal/diff"
	"stet/cli/internal/expand"
	"stet/cli/internal/findings"
	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
)

const maxExpandTokensCap = 4096

// ReviewHunk runs the review for a single hunk: loads system prompt, injects user
// intent if provided, expands hunk with enclosing function context when repoRoot
// and contextLimit are set (Phase 6.4), builds user prompt, calls Ollama Generate,
// parses the JSON response, and assigns finding IDs. generateOpts is passed to
// the Ollama API (may be nil for server defaults). userIntent, when non-nil and
// non-empty, injects branch and commit message into the "## User Intent" section.
// On malformed JSON it retries the Generate call once; on second parse failure
// returns an error.
func ReviewHunk(ctx context.Context, client *ollama.Client, model, stateDir string, hunk diff.Hunk, generateOpts *ollama.GenerateOptions, userIntent *prompt.UserIntent, repoRoot string, contextLimit int) ([]findings.Finding, error) {
	system, err := prompt.SystemPrompt(stateDir)
	if err != nil {
		return nil, fmt.Errorf("review: system prompt: %w", err)
	}
	if userIntent != nil && (userIntent.Branch != "" || userIntent.CommitMsg != "") {
		system = prompt.InjectUserIntent(system, userIntent.Branch, userIntent.CommitMsg)
	}
	if repoRoot != "" && contextLimit > 0 {
		maxExpand := contextLimit / 4
		if maxExpand > maxExpandTokensCap {
			maxExpand = maxExpandTokensCap
		}
		if expanded, expandErr := expand.ExpandHunk(repoRoot, hunk, maxExpand); expandErr == nil {
			hunk = expanded
		}
	}
	user := prompt.UserPrompt(hunk)

	raw, err := client.Generate(ctx, model, system, user, generateOpts)
	if err != nil {
		return nil, fmt.Errorf("review: generate: %w", err)
	}
	list, err := ParseFindingsResponse(raw)
	if err != nil {
		raw2, retryErr := client.Generate(ctx, model, system, user, generateOpts)
		if retryErr != nil {
			return nil, fmt.Errorf("review: parse failed then retry generate failed: %w", retryErr)
		}
		list, err = ParseFindingsResponse(raw2)
		if err != nil {
			return nil, fmt.Errorf("review: parse failed (after retry): %w", err)
		}
	}

	return AssignFindingIDs(list, hunk.FilePath)
}
