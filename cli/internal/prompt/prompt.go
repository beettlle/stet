// Package prompt provides system prompt loading (optimized file or default) and
// user prompt building from a diff hunk for the review LLM.
package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"stet/cli/internal/diff"
)

const optimizedPromptFilename = "system_prompt_optimized.txt"

// DefaultSystemPrompt instructs the model to perform code review and return a
// single JSON array of findings. Schema matches findings.Finding (without id;
// the tool assigns IDs). Used when .review/system_prompt_optimized.txt is absent.
const DefaultSystemPrompt = `You are a code reviewer. Review the provided code diff hunk and report issues.

Respond with a single JSON array of findings. Each finding is an object with:
- file (string, required): path to the file
- line (integer, optional if range is set): line number
- range (object, optional): { "start": n, "end": n } for a line span
- severity (string, required): one of "error" | "warning" | "info" | "nitpick"
- category (string, required): one of "bug" | "security" | "performance" | "style" | "maintainability" | "testing"
- message (string, required): review comment
- suggestion (string, optional): suggested fix
- cursor_uri (string, optional): file URI for deep linking

Return only the JSON array, no other text. Example: [{"file":"pkg.go","line":10,"severity":"warning","category":"style","message":"Use consistent naming"}]`

// SystemPrompt returns the system prompt for the review model. If
// stateDir/system_prompt_optimized.txt exists and is readable, its contents
// (trimmed) are returned; otherwise DefaultSystemPrompt is returned.
// Missing file (IsNotExist) returns default with nil error; any other read
// error (e.g. permission denied) is returned so the user can see it.
func SystemPrompt(stateDir string) (string, error) {
	if stateDir == "" {
		return DefaultSystemPrompt, nil
	}
	path := filepath.Join(stateDir, optimizedPromptFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSystemPrompt, nil
		}
		return "", fmt.Errorf("read optimized prompt: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// UserPrompt builds the user-facing prompt for one hunk: file path and the
// hunk content (context). Phase 3 uses only hunk content; no RAG or extra context.
func UserPrompt(hunk diff.Hunk) string {
	content := hunk.Context
	if content == "" {
		content = hunk.RawContent
	}
	if hunk.FilePath == "" {
		return content
	}
	return "File: " + hunk.FilePath + "\n\n" + content
}
