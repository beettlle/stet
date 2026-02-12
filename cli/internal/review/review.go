package review

import (
	"context"
	"fmt"

	"stet/cli/internal/diff"
	"stet/cli/internal/findings"
	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
)

// ReviewHunk runs the review for a single hunk: loads system prompt, builds user
// prompt, calls Ollama Generate, parses the JSON response, and assigns finding IDs.
// On malformed JSON it retries the Generate call once; on second parse failure
// returns an error.
func ReviewHunk(ctx context.Context, client *ollama.Client, model, stateDir string, hunk diff.Hunk) ([]findings.Finding, error) {
	system, err := prompt.SystemPrompt(stateDir)
	if err != nil {
		return nil, fmt.Errorf("review: system prompt: %w", err)
	}
	user := prompt.UserPrompt(hunk)

	raw, err := client.Generate(ctx, model, system, user)
	if err != nil {
		return nil, fmt.Errorf("review: generate: %w", err)
	}
	list, err := ParseFindingsResponse(raw)
	if err != nil {
		raw2, retryErr := client.Generate(ctx, model, system, user)
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
