// Package prompt provides system prompt loading (optimized file or default) and
// user prompt building from a diff hunk for the review LLM.
package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"stet/cli/internal/diff"
	"stet/cli/internal/rules"
	"stet/cli/internal/tokens"
)

const userIntentHeader = "## User Intent\n"

// UserIntent holds Git context (branch, last commit message) to inject into the prompt.
type UserIntent struct {
	Branch    string
	CommitMsg string
}

const optimizedPromptFilename = "system_prompt_optimized.txt"

// DefaultSystemPrompt instructs the model to perform defect-focused code review
// (System 2 / Chain-of-Thought) and return a single JSON array of findings.
// Schema matches findings.Finding (without id; the tool assigns IDs). Used when
// .review/system_prompt_optimized.txt is absent.
const DefaultSystemPrompt = `You are a Senior Defect Analyst. Review the provided code diff hunk using step-by-step verification. Your goal is to find bugs, security vulnerabilities, and performance issues. Do not comment on style unless it introduces a defect.

## User Intent
(Not provided.)

## Review steps (follow in order)
1. Logic: Check for logic errors (off-by-one, null/zero checks, control flow). Verify variables and functions exist before flagging: if a variable, function, or type is used in the hunk but its definition is not present in the hunk, assume it is valid. Do not report "undefined", "not declared", or "variable not found" for identifiers whose definition is outside the hunk.
2. Security: Check for injection risks, sensitive data exposure, unsafe use of inputs. Before reporting a security or robustness finding, trace back through the code and check for validation in the same function or block. If validation exists, do not report: (a) Path handling: before flagging path traversal or unsafe path construction (e.g. filepath.Join with user input), look for filepath.Rel, filepath.Clean, or path-under-root checks. (b) File operations: before flagging unbounded or unsafe file reads, look for size checks (e.g. Stat, size limits). (c) Indexing/slicing: before flagging potential panics from slice or array access, look for bounds or offset checks.
3. Performance: Check for expensive operations in loops, unnecessary allocations, blocking calls.
4. Output: Emit only high-confidence, actionable findings. Before outputting: if a finding is a nitpick or style-only and not a defect, discard it. Prefer fewer, high-confidence findings over volume.

Report only actionable issues: the developer should be able to apply the suggestion or fix the issue without reverting correct behavior. Do not suggest reverting intentional changes, adding code that already exists, or changing behavior that matches documented design.

Respond with a single JSON array of findings. Each finding is an object with:
- file (string, required): path to the file
- line (integer, optional if range is set): line number
- range (object, optional): { "start": n, "end": n } for a line span
- severity (string, required): one of "error" | "warning" | "info" | "nitpick"
- category (string, required): one of "bug" | "security" | "performance" | "style" | "maintainability" | "testing" | "documentation" | "design"
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

// InjectUserIntent replaces the "## User Intent" section in systemPrompt with
// the given branch and commitMsg. If both are empty, uses "(Not provided.)".
// If the section is missing, returns systemPrompt unchanged.
func InjectUserIntent(systemPrompt, branch, commitMsg string) string {
	idx := strings.Index(systemPrompt, userIntentHeader)
	if idx == -1 {
		return systemPrompt
	}
	body := "(Not provided.)"
	if branch != "" || commitMsg != "" {
		var b strings.Builder
		if branch != "" {
			b.WriteString("Branch: ")
			b.WriteString(branch)
			b.WriteByte('\n')
		}
		if commitMsg != "" {
			b.WriteString("Commit: ")
			b.WriteString(commitMsg)
		}
		body = strings.TrimSpace(b.String())
	}
	sectionStart := idx
	rest := systemPrompt[idx+len(userIntentHeader):]
	end := strings.Index(rest, "\n## ")
	var sectionEnd int
	if end == -1 {
		sectionEnd = len(systemPrompt)
	} else {
		sectionEnd = sectionStart + len(userIntentHeader) + end
	}
	replacement := userIntentHeader + body
	return systemPrompt[:sectionStart] + replacement + systemPrompt[sectionEnd:]
}

const projectReviewCriteriaHeader = "## Project review criteria\n"

// AppendCursorRules appends a "## Project review criteria" section to
// systemPrompt when rules apply to filePath. Matched rules are ordered with
// alwaysApply first; combined content is truncated to maxRuleTokens. Returns
// systemPrompt unchanged if rules is nil/empty or no rules match filePath.
func AppendCursorRules(systemPrompt string, ruleList []rules.CursorRule, filePath string, maxRuleTokens int) string {
	if len(ruleList) == 0 {
		return systemPrompt
	}
	matched := rules.FilterRules(ruleList, filePath)
	if len(matched) == 0 {
		return systemPrompt
	}
	// Order: alwaysApply first, then others.
	var always, rest []rules.CursorRule
	for _, r := range matched {
		if r.AlwaysApply {
			always = append(always, r)
		} else {
			rest = append(rest, r)
		}
	}
	var combined strings.Builder
	appendContents(&combined, always)
	appendContents(&combined, rest)
	text := combined.String()
	if maxRuleTokens > 0 {
		text = truncateToTokenBudget(text, maxRuleTokens)
	}
	if text == "" {
		return systemPrompt
	}
	return systemPrompt + "\n\n" + projectReviewCriteriaHeader + text
}

func appendContents(b *strings.Builder, ruleList []rules.CursorRule) {
	for _, r := range ruleList {
		if r.Content != "" {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(r.Content)
		}
	}
}

func truncateToTokenBudget(s string, maxTokens int) string {
	if tokens.Estimate(s) <= maxTokens {
		return s
	}
	// Trim from end; ~4 chars per token.
	maxChars := maxTokens * 4
	if len(s) <= maxChars {
		return s
	}
	return s[:maxChars] + "\n\n[truncated]"
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
