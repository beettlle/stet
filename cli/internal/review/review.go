package review

import (
	"context"
	"fmt"
	"math"
	"strings"

	"stet/cli/internal/diff"
	"stet/cli/internal/expand"
	"stet/cli/internal/findings"
	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
	"stet/cli/internal/rag"
	"stet/cli/internal/rules"
	"stet/cli/internal/tokens"
	"stet/cli/internal/trace"
)

const maxExpandTokensCap = 4096

// effectiveRAGTokenCap returns the token cap for the RAG block this hunk.
// When contextLimit <= 0, returns ragMaxTokens unchanged (no adaptive cap).
// Otherwise: ragBudget = contextLimit - basePromptTokens - responseReserve;
// when ragMaxTokens == 0 use ragBudget, when ragMaxTokens > 0 use min(ragBudget, ragMaxTokens);
// result is clamped to >= 0.
func effectiveRAGTokenCap(contextLimit, basePromptTokens, responseReserve, ragMaxTokens int) int {
	if contextLimit <= 0 {
		return ragMaxTokens
	}
	// Use int64 to avoid overflow when basePromptTokens + responseReserve is large
	sum := int64(basePromptTokens) + int64(responseReserve)
	if sum > int64(contextLimit) || sum < 0 {
		return 0 // no budget when base + reserve exceeds or overflows
	}
	ragBudget64 := int64(contextLimit) - sum
	var ragBudget int
	if ragBudget64 <= 0 {
		ragBudget = 0
	} else if ragBudget64 > int64(math.MaxInt) {
		ragBudget = math.MaxInt
	} else {
		ragBudget = int(ragBudget64)
	}
	var effective int
	if ragMaxTokens == 0 {
		effective = ragBudget
	} else {
		effective = min(ragBudget, ragMaxTokens)
	}
	return max(0, effective)
}

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
		const previewLen = 2000
		if len(user) <= previewLen {
			traceOut.Printf("len=%d\n%s\n", len(user), user)
		} else {
			traceOut.Printf("len=%d (first %d chars)\n%s\n[truncated]\n", len(user), previewLen, user[:previewLen])
		}
	}
	// Token estimation is intentional per-hunk for accuracy (each hunk may have different expand/RAG).
	basePromptTokens := tokens.Estimate(system + "\n" + user)
	effectiveRAGTokens := effectiveRAGTokenCap(contextLimit, basePromptTokens, tokens.DefaultResponseReserve, ragMaxTokens)
	var defs []rag.Definition
	doRAG := repoRoot != "" && ragMaxDefs > 0 && (contextLimit <= 0 || effectiveRAGTokens > 0)
	if doRAG {
		var ragErr error
		defs, ragErr = rag.ResolveSymbols(ctx, repoRoot, hunk.FilePath, hunk.RawContent, rag.ResolveOptions{MaxDefinitions: ragMaxDefs, MaxTokens: effectiveRAGTokens})
		if ragErr == nil {
			user = prompt.AppendSymbolDefinitions(user, defs, effectiveRAGTokens)
		}
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("RAG")
		// Traced value matches what is passed to ResolveSymbols and AppendSymbolDefinitions.
		traceOut.Printf("effective_rag_tokens=%d definitions=%d\n", effectiveRAGTokens, len(defs))
		for _, d := range defs {
			traceOut.Printf("  %s %s:%d %s\n", d.Symbol, d.File, d.Line, d.Signature)
		}
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("LLM request")
		temp, numCtx := 0.0, 0
		if generateOpts != nil {
			temp, numCtx = generateOpts.Temperature, generateOpts.NumCtx
		}
		traceOut.Printf("model=%s temperature=%g num_ctx=%d system_len=%d user_len=%d\n", model, temp, numCtx, len(system), len(user))
	}
	result, err := client.Generate(ctx, model, system, user, generateOpts)
	if err != nil {
		return nil, fmt.Errorf("review: generate: %w", err)
	}
	// API returns non-nil result on success; defensive check.
	if result == nil {
		return nil, fmt.Errorf("review: generate: unexpected nil result")
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("LLM response (raw)")
		traceOut.Printf("model=%s prompt_eval_count=%d eval_count=%d eval_duration=%d\n", result.Model, result.PromptEvalCount, result.EvalCount, result.EvalDuration)
		traceOut.Printf("%s\n", result.Response)
	}
	list, err := ParseFindingsResponse(result.Response)
	if err != nil {
		result2, retryErr := client.Generate(ctx, model, system, user, generateOpts)
		if retryErr != nil {
			return nil, fmt.Errorf("review: parse failed then retry generate failed: %w", retryErr)
		}
		// API returns non-nil result on success; defensive check.
		if result2 == nil {
			return nil, fmt.Errorf("review: generate retry: unexpected nil result")
		}
		if traceOut != nil && traceOut.Enabled() {
			traceOut.Section("LLM response (raw) retry")
			traceOut.Printf("model=%s prompt_eval_count=%d eval_count=%d eval_duration=%d\n", result2.Model, result2.PromptEvalCount, result2.EvalCount, result2.EvalDuration)
			traceOut.Printf("%s\n", result2.Response)
		}
		list, err = ParseFindingsResponse(result2.Response)
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
