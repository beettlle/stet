// Package commitmsg provides LLM-based git commit message generation from a diff.
package commitmsg

import (
	"context"
	"errors"
	"strings"

	"stet/cli/internal/ollama"
)

const maxDiffChars = 32 * 1024

// SystemPrompt instructs the model to produce a conventional git commit message.
const SystemPrompt = `You generate conventional git commit messages from a unified diff.
Output only the commit message, no other text or explanation.
Format:
- First line: short summary, 50 characters or less. Use imperative mood (e.g. "Add feature" not "Added feature"). Optionally prefix with scope (e.g. "cli: add commitmsg command").
- Blank line.
- Then a longer description if needed, wrapped at 72 characters.
Do not wrap the first line. Do not use markdown, code blocks, or quotes.`

// Suggest asks the model to generate a commit message for the given unified diff.
// diff is the output of git diff (staged and/or unstaged). opts may be nil; a lower
// temperature (e.g. 0.2) is recommended for more deterministic messages.
func Suggest(ctx context.Context, client *ollama.Client, model, diff string, opts *ollama.GenerateOptions) (string, error) {
	if client == nil {
		return "", errors.New("commitmsg: nil client")
	}
	if len(diff) > maxDiffChars {
		diff = diff[:maxDiffChars] + "\n\n[truncated for context]"
	}
	res, err := client.GeneratePlain(ctx, model, SystemPrompt, diff, opts)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Response), nil
}
