// Package prompt provides system prompt loading (optimized file or default) and
// user prompt building from a diff hunk for the review LLM.
package prompt

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"stet/cli/internal/diff"
	"stet/cli/internal/erruser"
	"stet/cli/internal/rag"
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

The user content is a unified diff hunk: lines starting with "-" are removed, lines starting with "+" are added. Review the change and the resulting code (the + side) for defects. Do not report issues that exist only in the removed lines (-) and are already fixed by the added lines (+). Report only issues in the resulting code or introduced by the change.

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
- confidence (number, required): your certainty 0.0–1.0 (1.0 = definite defect, lower = possible issue)
- message (string, required): review comment
- suggestion (string, optional): suggested fix
- cursor_uri (string, optional): file URI for deep linking

Set confidence to how sure you are for each finding. Return only the JSON array, no other text. Example: [{"file":"pkg.go","line":10,"severity":"warning","category":"style","confidence":0.9,"message":"Use consistent naming"}]`

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
		return "", erruser.New("Could not read system prompt file.", err)
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

const symbolDefinitionsHeader = "## Symbol definitions (for context)\n\n"

// AppendSymbolDefinitions appends a section with symbol definitions (signature +
// optional docstring) to the user prompt. Used by RAG-lite (Sub-phase 6.8).
// If defs is nil or empty, returns userPrompt unchanged. If maxTokens > 0,
// the appended block is truncated to fit the token budget.
func AppendSymbolDefinitions(userPrompt string, defs []rag.Definition, maxTokens int) string {
	if len(defs) == 0 {
		return userPrompt
	}
	var b strings.Builder
	for i, d := range defs {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("(")
		b.WriteString("File: ")
		b.WriteString(d.File)
		b.WriteString(", Line: ")
		b.WriteString(strconv.Itoa(d.Line))
		b.WriteString(")")
		if d.Docstring != "" {
			b.WriteString("\n\n")
			b.WriteString(d.Docstring)
		}
		b.WriteString("\n\n```\n")
		b.WriteString(d.Signature)
		b.WriteString("\n```")
	}
	text := b.String()
	if maxTokens > 0 {
		text = truncateToTokenBudget(text, maxTokens)
	}
	return userPrompt + "\n\n" + symbolDefinitionsHeader + text
}

// Shadow holds a dismissed finding's ID and the prompt context (hunk content)
// for optional negative few-shot injection (Sub-phase 6.9).
type Shadow struct {
	FindingID     string
	PromptContext string
}

const (
	maxPromptShadowsInject = 5
	maxShadowContextChars = 512
)

// AppendPromptShadows appends a "## Negative examples (do not report)" section
// with up to maxPromptShadowsInject most recent shadows, each truncated to
// maxShadowContextChars when injecting. Returns system unchanged if shadows is nil/empty.
func AppendPromptShadows(system string, shadows []Shadow) string {
	if len(shadows) == 0 {
		return system
	}
	n := len(shadows)
	if n > maxPromptShadowsInject {
		n = maxPromptShadowsInject
	}
	var b strings.Builder
	b.WriteString("\n\n## Negative examples (do not report)\n\n")
	b.WriteString("The following code contexts previously produced findings that the user dismissed. Do not report similar issues for similar code.\n\n")
	for i := len(shadows) - n; i < len(shadows); i++ {
		ctx := shadows[i].PromptContext
		if len(ctx) > maxShadowContextChars {
			ctx = ctx[:maxShadowContextChars] + "\n[truncated]"
		}
		b.WriteString("Context (dismissed):\n```\n")
		b.WriteString(ctx)
		b.WriteString("\n```\n\n")
	}
	return system + b.String()
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
