package review

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"stet/cli/internal/diff"
	"stet/cli/internal/expand"
	"stet/cli/internal/findings"
	"stet/cli/internal/minify"
	"stet/cli/internal/ollama"
	"stet/cli/internal/prompt"
	"stet/cli/internal/rag"
	"stet/cli/internal/rules"
	"stet/cli/internal/tokens"
	"stet/cli/internal/trace"
)

const maxExpandTokensCap = 4096

// maxRAGTokensWhenContextLarge caps per-hunk RAG tokens when the model context limit
// is very large so that a single hunk cannot consume most of a 128k/256k context window.
const maxRAGTokensWhenContextLarge = 65536

// HunkUsage holds per-hunk token and duration from Ollama (for history/analytics).
type HunkUsage struct {
	PromptEvalCount  int   // Prompt tokens for this hunk.
	EvalCount        int   // Completion tokens for this hunk.
	EvalDurationNs   int64 // Evaluation duration in nanoseconds (Ollama eval_duration).
}

// effectiveRAGTokenCap returns the token cap for the RAG block this hunk.
// When contextLimit <= 0, returns ragMaxTokens unchanged (no adaptive cap).
// Otherwise: ragBudget = contextLimit - basePromptTokens - responseReserve;
// when ragMaxTokens == 0 use ragBudget, when ragMaxTokens > 0 use min(ragBudget, ragMaxTokens);
// result is clamped to >= 0. For very large context limits (e.g. 128k/256k), the cap is
// further bounded so a single hunk cannot consume almost the full context window.
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
	// When contextLimit is very large (e.g. 128k/256k), prevent a single hunk
	// from consuming almost the entire context window via RAG alone.
	if contextLimit > 65536 && effective > maxRAGTokensWhenContextLarge {
		effective = maxRAGTokensWhenContextLarge
	}
	return max(0, effective)
}

// maxSuppressionExamplesPerHunk caps how many suppression examples can be
// appended per hunk when using a token budget (per-hunk suppression).
const maxSuppressionExamplesPerHunk = 30

// maxSuppressionTokensCap bounds the token budget used for suppression examples
// so that the suppression block stays small even when contextLimit is very large.
const maxSuppressionTokensCap = 8192

// PrepareHunkPrompt builds the system and user prompts for a single hunk from a
// pre-built systemBase (SystemPrompt + InjectUserIntent + AppendPromptShadows +
// AppendNitpickyInstructions). It appends Cursor rules for the hunk's file,
// expands the hunk, optionally minifies (Go), builds the user prompt (unified or
// search-replace when useSearchReplaceFormat), appends up to maxSuppressionExamplesPerHunk
// suppression examples when suppressionExamples is non-empty and contextLimit > 0
// (only as many as fit in the remaining token budget), and runs RAG when enabled.
// When ragCallGraphEnabled is true and the file is Go, call-graph (callers/callees)
// is resolved and appended to the middle block; token cap is ragCallGraphMaxTokens
// or half of effectiveRAGTokens when 0. Used by the pipeline to prepare the next hunk.
func PrepareHunkPrompt(ctx context.Context, systemBase string, hunk diff.Hunk, ruleList []rules.CursorRule, repoRoot string, contextLimit int, ragMaxDefs, ragMaxTokens int, ragCallGraphEnabled bool, ragCallersMax, ragCalleesMax, ragCallGraphMaxTokens int, useSearchReplaceFormat bool, suppressionExamples []string, traceOut *trace.Tracer) (system, user string, err error) {
	system = prompt.AppendCursorRules(systemBase, ruleList, hunk.FilePath, rules.MaxRuleTokens)
	if useSearchReplaceFormat {
		system = prompt.AppendSearchReplaceFormatNote(system)
	}
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
	ext := filepath.Ext(hunk.FilePath)
	if ext == ".go" || ext == ".rs" {
		var minified string
		if ext == ".go" {
			minified = minify.MinifyGoHunkContent(hunk.RawContent)
		} else {
			minified = minify.MinifyRustHunkContent(hunk.RawContent)
		}
		if strings.Contains(hunk.Context, "## Diff hunk") {
			hunk.Context = strings.TrimSuffix(hunk.Context, hunk.RawContent) + minified
		} else {
			hunk.Context = minified
		}
	}
	if useSearchReplaceFormat {
		user = prompt.UserPromptSearchReplace(hunk)
	} else {
		user = prompt.UserPrompt(hunk)
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("User prompt")
		const previewLen = 2000
		if len(user) <= previewLen {
			traceOut.Printf("len=%d\n%s\n", len(user), user)
		} else {
			traceOut.Printf("len=%d (first %d chars)\n%s\n[truncated]\n", len(user), previewLen, user[:previewLen])
		}
	}
	// Per-hunk suppression: append only as many examples as fit in the remaining token budget.
	if len(suppressionExamples) > 0 && contextLimit > 0 {
		baseNoSupp := tokens.Estimate(system + "\n" + user)
		// Use int64 to avoid overflow when baseNoSupp + DefaultResponseReserve exceeds contextLimit.
		suppSum := int64(baseNoSupp) + int64(tokens.DefaultResponseReserve)
		suppressionBudget := 0
		if suppSum >= 0 && suppSum < int64(contextLimit) {
			suppressionBudget = int(int64(contextLimit) - suppSum)
		}
		if suppressionBudget > 0 {
			if suppressionBudget > maxSuppressionTokensCap {
				suppressionBudget = maxSuppressionTokensCap
			}
			maxN := maxSuppressionExamplesPerHunk
			if len(suppressionExamples) < maxN {
				maxN = len(suppressionExamples)
			}
			maxExamples := 0
			for n := 1; n <= maxN; n++ {
				if prompt.EstimateSuppressionBlock(suppressionExamples, n) <= suppressionBudget {
					maxExamples = n
				} else {
					break
				}
			}
			if maxExamples > 0 {
				system = prompt.AppendSuppressionExamples(system, suppressionExamples[len(suppressionExamples)-maxExamples:])
				if traceOut != nil && traceOut.Enabled() {
					traceOut.Section("Suppression")
					traceOut.Printf("Suppression: %d examples (budget %d tokens)\n", maxExamples, suppressionBudget)
				}
			}
		}
	}
	basePromptTokens := tokens.Estimate(system + "\n" + user)
	effectiveRAGTokens := effectiveRAGTokenCap(contextLimit, basePromptTokens, tokens.DefaultResponseReserve, ragMaxTokens)
	doRAG := repoRoot != "" && ragMaxDefs > 0 && (contextLimit <= 0 || effectiveRAGTokens > 0)
	var symbolDefsBlock string
	if doRAG {
		var defs []rag.Definition
		defs, err = rag.ResolveSymbols(ctx, repoRoot, hunk.FilePath, hunk.RawContent, rag.ResolveOptions{MaxDefinitions: ragMaxDefs, MaxTokens: effectiveRAGTokens})
		if err != nil {
			return "", "", fmt.Errorf("review: RAG resolve: %w", err)
		}
		symbolDefsBlock = prompt.FormatSymbolDefinitions(defs, effectiveRAGTokens)
		if traceOut != nil && traceOut.Enabled() {
			traceOut.Section("RAG")
			traceOut.Printf("effective_rag_tokens=%d definitions=%d\n", effectiveRAGTokens, len(defs))
			for _, d := range defs {
				traceOut.Printf("  %s %s:%d %s\n", d.Symbol, d.File, d.Line, d.Signature)
			}
		}
	} else if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("RAG")
		traceOut.Printf("effective_rag_tokens=%d definitions=0\n", effectiveRAGTokens)
	}
	// Call-graph (callers/callees) for Go only when enabled. Token cap: config or half of RAG budget.
	middleBlock := symbolDefsBlock
	doCallGraph := repoRoot != "" && ragCallGraphEnabled && filepath.Ext(hunk.FilePath) == ".go"
	if doCallGraph {
		callGraphTokenCap := ragCallGraphMaxTokens
		if callGraphTokenCap <= 0 {
			callGraphTokenCap = effectiveRAGTokens / 2
		}
		cgResult, cgErr := rag.ResolveCallGraph(ctx, repoRoot, hunk.FilePath, hunk.RawContent, rag.CallGraphOptions{
			CallersMax: ragCallersMax,
			CalleesMax: ragCalleesMax,
			MaxTokens:  callGraphTokenCap,
		})
		if cgErr == nil && cgResult != nil && (len(cgResult.Callers) > 0 || len(cgResult.Callees) > 0) {
			callGraphBlock := prompt.FormatCallGraph(cgResult.Callers, cgResult.Callees, callGraphTokenCap)
			if callGraphBlock != "" {
				if middleBlock != "" {
					middleBlock = middleBlock + "\n\n" + callGraphBlock
				} else {
					middleBlock = callGraphBlock
				}
				if traceOut != nil && traceOut.Enabled() {
					traceOut.Section("RAG call-graph")
					traceOut.Printf("callers=%d callees=%d\n", len(cgResult.Callers), len(cgResult.Callees))
				}
			}
		}
	}
	if middleBlock != "" {
		user = prompt.UserPromptWithRAGPlacement(user, middleBlock)
	}
	return system, user, nil
}

// ProcessReviewResponse parses the LLM response, retries Generate once on parse
// failure, then assigns finding IDs. Callers use this after client.Generate.
func ProcessReviewResponse(ctx context.Context, result *ollama.GenerateResult, hunk diff.Hunk, client *ollama.Client, model, system, user string, generateOpts *ollama.GenerateOptions, traceOut *trace.Tracer) ([]findings.Finding, *HunkUsage, error) {
	if result == nil {
		return nil, nil, fmt.Errorf("review: process response: nil result")
	}
	usage := &HunkUsage{PromptEvalCount: result.PromptEvalCount, EvalCount: result.EvalCount, EvalDurationNs: result.EvalDuration}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("LLM response (raw)")
		traceOut.Printf("model=%s prompt_eval_count=%d eval_count=%d eval_duration=%d\n", result.Model, result.PromptEvalCount, result.EvalCount, result.EvalDuration)
		traceOut.Printf("%s\n", result.Response)
	}
	var parseOpts []ParseOptions
	if traceOut != nil && traceOut.Enabled() {
		parseOpts = []ParseOptions{{
			OnDropped: func(index int, reason string) {
				traceOut.Printf("dropped finding %d: %s\n", index, reason)
			},
		}}
	}
	list, err := ParseFindingsResponse(result.Response, parseOpts...)
	if err != nil {
		result2, retryErr := client.Generate(ctx, model, system, user, generateOpts)
		if retryErr != nil {
			return nil, nil, fmt.Errorf("review: parse failed then retry generate failed: %w", retryErr)
		}
		if result2 == nil {
			return nil, nil, fmt.Errorf("review: generate retry: unexpected nil result")
		}
		if traceOut != nil && traceOut.Enabled() {
			traceOut.Section("LLM response (raw) retry")
			traceOut.Printf("model=%s prompt_eval_count=%d eval_count=%d eval_duration=%d\n", result2.Model, result2.PromptEvalCount, result2.EvalCount, result2.EvalDuration)
			traceOut.Printf("%s\n", result2.Response)
		}
		usage = &HunkUsage{PromptEvalCount: result2.PromptEvalCount, EvalCount: result2.EvalCount, EvalDurationNs: result2.EvalDuration}
		list, err = ParseFindingsResponse(result2.Response, parseOpts...)
		if err != nil {
			return nil, nil, fmt.Errorf("review: parse failed (after retry): %w", err)
		}
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("Parse")
		traceOut.Printf("Parsed %d findings\n", len(list))
	}
	list, err = AssignFindingIDs(list, hunk.FilePath)
	if err != nil {
		return nil, nil, err
	}
	return list, usage, nil
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
// (Sub-phase 6.8); zero ragMaxDefs disables it. ragCallGraphEnabled and
// ragCallersMax/ragCalleesMax/ragCallGraphMaxTokens control optional call-graph (Go only).
// When nitpicky is true, nitpicky-mode instructions are appended.
// suppressionExamples, when non-nil and non-empty, are applied per-hunk (as many as fit in the token budget).
func ReviewHunk(ctx context.Context, client *ollama.Client, model, stateDir string, hunk diff.Hunk, generateOpts *ollama.GenerateOptions, userIntent *prompt.UserIntent, ruleList []rules.CursorRule, repoRoot string, contextLimit int, ragMaxDefs, ragMaxTokens int, ragCallGraphEnabled bool, ragCallersMax, ragCalleesMax, ragCallGraphMaxTokens int, promptShadows []prompt.Shadow, nitpicky bool, useSearchReplaceFormat bool, suppressionExamples []string, traceOut *trace.Tracer) ([]findings.Finding, *HunkUsage, error) {
	systemBase, err := prompt.SystemPrompt(stateDir)
	if err != nil {
		return nil, nil, fmt.Errorf("review: system prompt: %w", err)
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("System prompt")
		traceOut.Printf("source=%s len=%d\n", prompt.SystemPromptSource(stateDir), len(systemBase))
	}
	if userIntent != nil && (userIntent.Branch != "" || userIntent.CommitMsg != "") {
		systemBase = prompt.InjectUserIntent(systemBase, userIntent.Branch, userIntent.CommitMsg)
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("User intent")
		if userIntent != nil && (userIntent.Branch != "" || userIntent.CommitMsg != "") {
			traceOut.Printf("branch=%s commit=%s\n", userIntent.Branch, userIntent.CommitMsg)
		} else {
			traceOut.Printf("(not provided)\n")
		}
	}
	if len(promptShadows) > 0 {
		systemBase = prompt.AppendPromptShadows(systemBase, promptShadows)
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("Prompt shadows")
		traceOut.Printf("count=%d\n", len(promptShadows))
	}
	if nitpicky {
		systemBase = prompt.AppendNitpickyInstructions(systemBase)
	}
	if traceOut != nil && traceOut.Enabled() {
		traceOut.Section("Nitpicky")
		if nitpicky {
			traceOut.Printf("Nitpicky: enabled\n")
		} else {
			traceOut.Printf("Nitpicky: disabled\n")
		}
	}
	system, user, err := PrepareHunkPrompt(ctx, systemBase, hunk, ruleList, repoRoot, contextLimit, ragMaxDefs, ragMaxTokens, ragCallGraphEnabled, ragCallersMax, ragCalleesMax, ragCallGraphMaxTokens, useSearchReplaceFormat, suppressionExamples, traceOut)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, fmt.Errorf("review: generate: %w", err)
	}
	return ProcessReviewResponse(ctx, result, hunk, client, model, system, user, generateOpts, traceOut)
}
