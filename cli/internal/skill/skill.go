// Package skill provides the Agent Skills SKILL.md content for stet integration
// with LLM agents (Cursor, Claude Code, Codex, skills.sh, etc.).
package skill

// SKILL returns the full SKILL.md content (YAML frontmatter + Markdown body)
// for the stet-integration Agent Skill. The version string is embedded in
// frontmatter metadata (e.g. from version.String()).
func SKILL(version string) string {
	if version == "" {
		version = "1.0"
	}
	// Use double-quoted string so markdown backticks (e.g. `stet doctor`) do not break the literal.
	return "---\n" +
		"name: stet-integration\n" +
		"description: Run and triage stet code reviews from chat. Use when the user asks to run stet, dismiss findings, start or finish a review, list findings, or check review status. Keywords: stet, code review, dismiss, findings, triage, Ollama.\n" +
		"metadata:\n" +
		"  version: \"" + version + "\"\n" +
		"---\n\n" +
		"# Stet Integration\n\n" +
		"When the user asks to dismiss findings, run stet, or triage reviews, use the commands and dismiss reasons below.\n\n" +
		"## When to Use This Skill\n\n" +
		"Activate this skill when the user wants to:\n\n" +
		"- Run a code review with stet (`stet start`, `stet run`)\n" +
		"- Dismiss or triage findings (`stet dismiss`)\n" +
		"- Start or finish a review session (`stet start`, `stet finish`)\n" +
		"- List findings or check status (`stet list`, `stet status`)\n" +
		"- Verify environment (`stet doctor`) or clean up worktrees (`stet cleanup`)\n\n" +
		"Do not activate for generic \"review my code\" unless the user clearly means stet or the project uses stet.\n\n" +
		"## Commands\n\n" +
		"| Command | Description |\n" +
		"|---------|-------------|\n" +
		"| `stet doctor` | Verify Ollama, Git, models |\n" +
		"| `stet start [ref]` | Start review from baseline (default ref HEAD) |\n" +
		"| `stet run` | Re-run incremental review |\n" +
		"| `stet rerun` | Re-run full review over all hunks; use --replace to overwrite findings |\n" +
		"| `stet finish` | Persist state, clean up |\n" +
		"| `stet list` | List active findings with IDs (for stet dismiss) |\n" +
		"| `stet status` | Show session status |\n" +
		"| `stet dismiss <id> [reason]` | Mark finding dismissed; prefer giving a reason |\n" +
		"| `stet cleanup` | Remove orphan stet worktrees |\n" +
		"| `stet optimize` | Run optional optimizer (history → optimized prompt) |\n\n" +
		"## Dismiss Reasons\n\n" +
		"Valid values for `stet dismiss <id> <reason>`: `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`.\n\n" +
		"| Reason | Use when |\n" +
		"|--------|----------|\n" +
		"| **false_positive** | Not a real issue; model misread; redundant or low-signal nit. |\n" +
		"| **already_correct** | Code already correct; concern addressed; finding about removed lines fixed by the change. |\n" +
		"| **wrong_suggestion** | Suggestion wrong or harmful (wrong tool, would make code inconsistent). |\n" +
		"| **out_of_scope** | Wrong scope (e.g. generated files, meta/curated docs). |\n\n" +
		"**Quick pick:** Worse/inconsistent → `wrong_suggestion`; wrong scope → `out_of_scope`; already correct → `already_correct`; else → `false_positive`.\n\n" +
		"## Rules\n\n" +
		"1. Run stet from the **repository root** (or ensure the shell cwd is the repo root before invoking stet).\n" +
		"2. When suggesting or running `stet dismiss`, **prefer giving a reason**; use only the four valid values above.\n" +
		"3. Do not dismiss without a reason when the user said \"not useful\"—choose the matching reason (e.g. false_positive).\n" +
		"4. If you are acting as the **review model** (e.g. stet injects this skill for the reviewer): report only actionable findings; prefer fewer, high-confidence issues; do not report issues that exist only in removed lines (-) and are already fixed by the added lines (+).\n" +
		"5. On `stet start` failure, surface the **recovery hint** from stderr (e.g. commit or stash changes, or run `stet finish` then `stet start` again).\n\n" +
		"## Examples\n\n" +
		"**User:** \"Dismiss finding abc123 as false positive.\"\n" +
		"**Action:** Run `stet dismiss abc123 false_positive`.\n\n" +
		"**User:** \"Run a review from HEAD.\"\n" +
		"**Action:** Run `stet start` or `stet start HEAD`.\n\n" +
		"**User:** \"List current stet findings.\"\n" +
		"**Action:** Run `stet list` (or `stet status --ids`).\n\n" +
		"## Cursor Extension\n\n" +
		"The Cursor extension shows findings in a panel, supports \"Copy for chat,\" and can run \"Finish review.\" It runs the CLI and displays results; the same commands and dismiss reasons apply.\n"
}
