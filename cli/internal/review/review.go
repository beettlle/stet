package review

import (
	"context"
	"fmt"
	"strings"

	"stet/cli/internal/diff"
	"stet/cli/internal/expand"
	"stet/cli/internal/findings"
	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
	"stet/cli/internal/rag"
	"stet/cli/internal/rules"
	"stet/cli/internal/trace"
)

const maxExpandTokensCap = 4096

// ReviewHunk runs the review for a single hunk: loads system prompt, injects user
// intent if provided, appends Cursor rules that apply to the file (Phase 6.6),
// appends prompt shadows as negative examples when provided (Sub-phase 6.9),
// expands hunk with enclosing function context when repoRoot and contextLimit are
// set (Phase 6.4), builds user prompt, calls Ollama Generate, parses the JSON
// response, and assigns finding IDs. generateOpts is passed to the Ollama API
// (may be nil for server defaults). userIntent, when non-nil and non-empty,
// injects branch and commit message into the "## User Intent" section. ruleList
// is from rules.LoadRules (may be nil when .cursor/rules is absent).
// promptShadows, when non-empty, are appended as negative few-shot examples.
// On malformed JSON it retries the Generate call once; on second parse failure
// returns an error. ragMaxDefs and ragMaxTokens control RAG-lite symbol lookup
// (Sub-phase 6.8); zero ragMaxDefs disables it. When nitpicky is true, nitpicky-mode
// instructions are appended so the model reports style, typos, and convention violations.
func ReviewHunk(ctx context.Context, client *ollama.Client, model, stateDir string, hunk diff.Hunk, generateOpts *ollama.GenerateOptions, userIntent *prompt.UserIntent, ruleList []rules.CursorRule, repoRoot string, contextLimit int, ragMaxDefs, ragMaxTokens int, promptShadows []prompt.Shadow, nitpicky bool, traceOut *trace.Tracer) ([]findings.Finding, error) {
	system, err := prompt.SystemPrompt(stateDir)
	if err != nil {
		return nil, fmt.Errorf("review: system prompt: %w", err)
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("System prompt")
		traceOut.Printf("source=%s len=%d\n", prompt.SystemPromptSource(stateDir), len(system))
	}
	if userIntent != nil && (userIntent.Branch != "" || userIntent.CommitMsg != "") {
		system = prompt.InjectUserIntent(system, userIntent.Branch, userIntent.CommitMsg)
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("User intent")
		if userIntent != nil && (userIntent.Branch != "" || userIntent.CommitMsg != "") {
			traceOut.Printf("branch=%s commit=%s\n", userIntent.Branch, userIntent.CommitMsg)
		} else {
			traceOut.Printf("(not provided)\n")
		}
	}
	system = prompt.AppendCursorRules(system, ruleList, hunk.FilePath, rules.MaxRuleTokens)
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("Cursor rules")
		matched := rules.FilterRules(ruleList, hunk.FilePath)
		var names []string
		for _, r := range matched {
			if r.Source != "" {
				names = append(names, r.Source)
			}
		}
		if len(names) > 0 {
			traceOut.Printf("applied: %s\n", strings.Join(names, ", "))
		} else {
			traceOut.Printf("(none matched)\n")
		}
		traceOut.Printf("AGENTS.md: not used by stet (only .cursor/rules/ used).\n")
	}
	if len(promptShadows) > 0 {
		system = prompt.AppendPromptShadows(system, promptShadows)
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("Prompt shadows")
		traceOut.Printf("count=%d\n", len(promptShadows))
	}
	if nitpicky {
		system = prompt.AppendNitpickyInstructions(system)
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("Nitpicky")
		if nitpicky {
			traceOut.Printf("Nitpicky: enabled\n")
		} else {
			traceOut.Printf("Nitpicky: disabled\n")
		}
	}
	expandApplied := false
	if repoRoot != "" && contextLimit > 0 {
		maxExpand := contextLimit / 4
		if maxExpand > maxExpandTokensCap {
			maxExpand = maxExpandTokensCap
		}
		if expanded, expandErr := expand.ExpandHunk(repoRoot, hunk, maxExpand); expandErr == nil {
			hunk = expanded
			expandApplied = true
		}
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("Expand")
		if expandApplied {
			traceOut.Printf("Expand: applied\n")
		} else {
			traceOut.Printf("Expand: not applied\n")
		}
	}
	user := prompt.UserPrompt(hunk)
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("User prompt")
		const previewLen = 500
		if len(user) <= previewLen {
			traceOut.Printf("len=%d\n%s\n", len(user), user)
		} else {
			traceOut.Printf("len=%d (first %d chars)\n%s\n[truncated]\n", len(user), previewLen, user[:previewLen])
		}
	}
	var defs []rag.Definition
	if repoRoot != "" && ragMaxDefs > 0 {
		var ragErr error
		defs, ragErr = rag.ResolveSymbols(ctx, repoRoot, hunk.FilePath, hunk.RawContent, rag.ResolveOptions{MaxDefinitions: ragMaxDefs, MaxTokens: ragMaxTokens})
		if ragErr == nil {
			user = prompt.AppendSymbolDefinitions(user, defs, ragMaxTokens)
		}
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("RAG")
		traceOut.Printf("definitions=%d\n", len(defs))
		for _, d := range defs {
			traceOut.Printf("  %s %s:%d %s\n", d.Symbol, d.File, d.Line, d.Signature)
		}
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("LLM request")
		traceOut.Printf("system_len=%d user_len=%d\n", len(system), len(user))
	}
	raw, err := client.Generate(ctx, model, system, user, generateOpts)
	if err != nil {
		return nil, fmt.Errorf("review: generate: %w", err)
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("LLM response (raw)")
		traceOut.Printf("%s\n", raw)
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
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("Parse")
		traceOut.Printf("Parsed %d findings\n", len(list))
	}
	return AssignFindingIDs(list, hunk.FilePath)
}
