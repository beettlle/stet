# Efficacy Tests for Stet

This document defines two categories of tests for evaluating Stet's code review quality, model performance, and user experience:

1. **Automated tests** — Scriptable tests that a coding LLM can implement without human judgment. Each entry includes inputs, outputs, schemas, CLI invocations, and implementation steps.

2. **Human tests** — Experiments that require human judgment, labeling, or subjective assessment. Each entry includes experiment design, a step-by-step protocol, sample size guidance, and analysis steps.

---

## Prerequisites and References

### CLI Contract

- **Output format**: Use `--output=json` or `--json` for machine-parseable JSON on stdout. See [cli-extension-contract.md](cli-extension-contract.md).
- **Streaming**: Use `--stream` with `--json` for NDJSON events (`progress`, `finding`, `done`).
- **Exit codes**: 0 = success, 1 = usage/error, 2 = Ollama unreachable.
- **JSON shape**: `{"findings": [...]}`; each finding has `id`, `file`, `line`, `range`, `severity`, `category`, `confidence`, `message`, `suggestion`, `cursor_uri`.

### Configuration

- **Model**: `STET_MODEL` or `.review/config.toml` `model = "..."`. Default: `qwen3-coder:30b`. See [cli/internal/config/config.go](../cli/internal/config/config.go).
- **RAG**: `STET_RAG_SYMBOL_MAX_DEFINITIONS` (default 10; 0 = disable) or `--rag-symbol-max-definitions=0` on `stet start` / `stet run`.
- **Temperature**: `STET_TEMPERATURE` (default 0.2); use 0 for deterministic runs.

### Schemas

- **Finding**: [cli/internal/findings/finding.go](../cli/internal/findings/finding.go) — `id`, `file`, `line`, `range`, `severity`, `category`, `confidence`, `message`, `suggestion`, `cursor_uri`.
- **History Record**: [cli/internal/history/schema.go](../cli/internal/history/schema.go) — `diff_ref`, `review_output`, `user_action.dismissed_ids`, `user_action.dismissals[]` with `finding_id` and `reason`.
- **Dismissal reasons**: `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`.

### State Paths

- `.review/session.json` — Session state (baseline, findings, dismissed_ids).
- `.review/history.jsonl` — One JSON object per line; appended on dismiss and finish.

### Commands

- `stet start [ref]` — Start review; default ref is HEAD.
- `stet run` — Incremental re-review.
- `stet finish` — End session, remove worktree.
- `stet dismiss <id> [reason]` — Mark finding as dismissed with optional reason.
- `stet status --ids` — List active finding IDs.
- `stet list` — Same as status --ids for active findings.

---

## Part 1: Automated Tests

Each automated test is specified so a coding LLM can implement it. Run all commands from the repository root.

---

### A1: Same-Diff Model Swap

**Purpose**: Compare two models on the same diff range. Measures finding counts, overlap (exact and fuzzy), and unique findings per model. Used to decide which model to adopt (e.g., qwen3-coder:30b vs qwen2.5-coder:32b).

**Prerequisites**: Repo has committed changes; clean worktree; both models pulled in Ollama; `stet` in PATH.

**Inputs**:
- `repo_root`: Path to Git repo.
- `ref`: Baseline ref (e.g., `HEAD~5`).
- `model_a`, `model_b`: Ollama model names (e.g., `qwen2.5-coder:32b`, `qwen3-coder:30b`).

**Algorithm**:
1. `cd repo_root`.
2. If session exists (e.g., `stet status` exits 0), run `stet finish`. If `stet status` exits 1 with "No active session", skip.
3. `export STET_MODEL=model_a`; run `stet start ref --json`; capture stdout; parse JSON; extract `findings`; save to `findings_a.json`. Redirect stderr to /dev/null or log.
4. `stet finish`.
5. `export STET_MODEL=model_b`; run `stet start ref --json`; capture stdout; parse JSON; extract `findings`; save to `findings_b.json`.
6. Compute: `count_a`, `count_b`; `overlap_exact` = findings with same `file` and `line` (or `range`) in both; `overlap_fuzzy` = findings with same `file` and similar `message` (e.g., normalized substring or cosine); `unique_a`, `unique_b` = findings only in A or B.
7. Output comparison JSON.

**Output format**:
```json
{
  "model_a": "qwen2.5-coder:32b",
  "model_b": "qwen3-coder:30b",
  "ref": "HEAD~5",
  "count_a": 12,
  "count_b": 10,
  "overlap_exact": 5,
  "overlap_fuzzy": 6,
  "unique_a": 6,
  "unique_b": 4
}
```

**Pass/fail**: Report-only.

---

### A2: Actionability from History

**Purpose**: Compute actionability rate and per-reason breakdown from `.review/history.jsonl`. Actionability = share of findings not dismissed. Used to track how often users find findings useful over time.

**Prerequisites**: `.review/history.jsonl` exists and has records from finished sessions.

**Inputs**:
- `state_dir`: Path to `.review/` (default: `repo_root/.review`).
- `history_path`: `state_dir/history.jsonl`.

**Algorithm**:
1. Read `history_path` line by line.
2. For each line: parse JSON as history Record. Extract `review_output` (array of findings) and `user_action.dismissed_ids` (array of strings). Optionally extract `user_action.dismissals[]` for per-finding `reason`.
3. For each Record: `total = len(review_output)`; `dismissed = len(user_action.dismissed_ids)`; `actionability = 1 - dismissed/total` when total > 0.
4. Aggregate: mean actionability, total findings, total dismissed; per-reason counts from `dismissals[].reason` (false_positive, already_correct, wrong_suggestion, out_of_scope).
5. Note: `history.jsonl` does not store model name; to compare models, tag sessions externally (e.g., log model in a separate file keyed by `diff_ref` or timestamp).

**Output format**:
```json
{
  "records_processed": 20,
  "total_findings": 150,
  "total_dismissed": 45,
  "actionability_rate": 0.7,
  "reason_breakdown": {
    "false_positive": 25,
    "already_correct": 10,
    "wrong_suggestion": 7,
    "out_of_scope": 3
  }
}
```

**Pass/fail**: Report-only.

---

### A3: Latency and Throughput

**Purpose**: Measure wall-clock time per run and optionally per-hunk. Used to compare model speed and estimate review duration.

**Prerequisites**: Repo with non-empty diff; Ollama running; model pulled.

**Inputs**:
- `repo_root`, `ref` (e.g., `HEAD~3`).
- `model`: Ollama model name.

**Algorithm**:
1. `cd repo_root`; ensure clean session (`stet finish` if needed).
2. Run `stet start ref --json` with `STET_MODEL=model`; capture stderr and measure wall-clock (e.g., `time` command or process start/end timestamps).
3. Parse stderr for lines matching "Reviewing hunk N/M" to infer total hunks M and per-hunk timing if timestamps are available. Alternatively, use `--stream` and parse NDJSON; record timestamp of first `finding` and last `done` to compute duration.
4. Compute: `total_seconds`, `hunks_reviewed` (from progress messages or finding count proxy), `seconds_per_hunk = total_seconds / hunks_reviewed` when hunks > 0.

**Output format**:
```json
{
  "model": "qwen3-coder:30b",
  "ref": "HEAD~3",
  "total_seconds": 120.5,
  "hunks_reviewed": 8,
  "seconds_per_hunk": 15.06
}
```

**Pass/fail**: Report-only.

---

### A4: Repeatability

**Purpose**: Measure consistency of findings across multiple runs of the same model on the same diff. Low consistency suggests high variance; useful before comparing models.

**Prerequisites**: Repo with committed diff; Ollama running; model pulled.

**Inputs**:
- `repo_root`, `ref`.
- `model`: Ollama model name.
- `runs`: Number of runs (e.g., 3).
- Use `STET_TEMPERATURE=0` for deterministic sampling.

**Algorithm**:
1. For i in 1..runs: `stet finish` if needed; `STET_MODEL=model STET_TEMPERATURE=0 stet start ref --json`; parse stdout; save findings to `findings_i.json`; `stet finish`.
2. Build sets of finding keys: e.g., `(file, line, message_normalized)` or `id` if stable.
3. Compute Jaccard similarity between each pair of runs: `|A ∩ B| / |A ∪ B|`. Report mean and min Jaccard across pairs.
4. Optionally: count findings that appear in all runs vs only some runs.

**Output format**:
```json
{
  "model": "qwen3-coder:30b",
  "runs": 3,
  "jaccard_mean": 0.85,
  "jaccard_min": 0.78,
  "findings_in_all_runs": 6,
  "findings_in_some_runs": 4
}
```

**Pass/fail**: Report-only.

---

### A5: Category and Severity Distribution

**Purpose**: Compare the distribution of findings by `category` and `severity` across runs or models. Identifies if a model over-flags certain categories (e.g., maintainability) or under-flags others (e.g., security).

**Prerequisites**: Findings JSON from one or more runs (e.g., from `stet start --json` or saved output).

**Inputs**:
- One or more JSON files or objects with `findings` array.

**Algorithm**:
1. Parse each findings array.
2. For each finding: increment counter for `finding.category` and `finding.severity`.
3. Output counts per category and per severity; optionally normalized percentages.

**Output format**:
```json
{
  "source": "findings.json",
  "by_category": {
    "security": 2,
    "correctness": 5,
    "maintainability": 8,
    "best_practice": 3
  },
  "by_severity": {
    "error": 1,
    "warning": 6,
    "info": 10,
    "nitpick": 1
  }
}
```

**Pass/fail**: Report-only.

---

### A6: Finding-ID Stability

**Purpose**: Verify that the same hunk produces the same finding ID across runs. Important for dismissals and history to remain stable.

**Prerequisites**: Repo with committed diff; `STET_TEMPERATURE=0` for deterministic output.

**Inputs**:
- `repo_root`, `ref`, `model`.

**Algorithm**:
1. Run `stet start ref --json` twice with same model and temperature 0; `stet finish` between runs.
2. Parse both outputs; build maps `file:line -> [(id, message), ...]` (or use `id` as key if one finding per location).
3. For matching file:line (and optionally message): assert `id` is identical. Report any mismatches.

**Output format**:
```json
{
  "stable": true,
  "mismatches": [],
  "total_findings_run1": 5,
  "total_findings_run2": 5
}
```

**Pass/fail**: Fail if any `id` differs for same file:line+message.

---

### A7: Dry-Run Regression

**Purpose**: Ensure `--dry-run` produces valid JSON conforming to the findings schema. Used in CI when Ollama is not available.

**Prerequisites**: Repo with at least one hunk in diff; `stet` in PATH. Ollama not required.

**Inputs**:
- `repo_root`, `ref` (e.g., `HEAD~1`).

**Algorithm**:
1. `stet finish` if session exists.
2. Run `stet start ref --dry-run --json`; capture stdout; expect exit 0.
3. Parse JSON; assert top-level has `findings` array.
4. For each finding: assert required fields present (`file`, `severity`, `category`, `confidence`, `message`). Assert `severity` in allowed set; `category` in allowed set; `confidence` in [0, 1].
5. If diff has hunks: assert `findings` is non-empty (dry-run emits one finding per hunk typically).

**Output format**: Pass/fail plus optional schema validation report.

**Pass/fail**: Fail if JSON invalid, schema violated, or findings empty when hunks exist.

---

### A8: Multi-Run Aggregation

**Purpose**: Run the model N times and merge findings (union by file:line or by semantic similarity). Evaluates whether aggregation boosts recall (as in SWR-Bench). Optionally compare against ground-truth for precision/recall.

**Prerequisites**: Repo with diff; Ollama running; optional: ground-truth JSON with expected findings (file, line, message or id).

**Inputs**:
- `repo_root`, `ref`, `model`, `runs` (e.g., 3).
- Optional: `ground_truth.json` with `{"expected": [{"file": "...", "line": N, "message": "..."}]}`.

**Algorithm**:
1. Run `stet start ref --json` N times with same model; collect all findings.
2. Merge: union by `(file, line)` or by normalized message similarity to deduplicate.
3. If ground truth provided: match each expected item to merged findings (file:line match or message similarity); compute TP, FP, FN; precision = TP/(TP+FP), recall = TP/(TP+FN), F1.
4. Output merged count, and if ground truth: precision, recall, F1.

**Output format**:
```json
{
  "runs": 3,
  "merged_findings_count": 15,
  "single_run_avg_count": 10,
  "ground_truth_provided": true,
  "precision": 0.8,
  "recall": 0.75,
  "f1": 0.77
}
```

**Pass/fail**: Report-only; optional fail if F1 below threshold when ground truth provided.

---

### A9: Context-Level Tagging

**Purpose**: Infer the context level (diff, file, repo) required for each finding. Heuristic only; supports analysis of where models need more context.

**Prerequisites**: Findings JSON. Heuristic may be inaccurate; document limitations.

**Inputs**:
- Findings array; optionally the diff/hunk metadata (which files were in the diff).

**Algorithm**:
1. For each finding: if `file` is not in the list of files changed in the diff, tag as `repo` (cross-file). Else if the finding references symbols or lines outside the changed hunk (requires parsing message or suggestion), tag as `file`. Else tag as `diff`.
2. Simpler heuristic: all findings in changed files → `diff`; findings in other files → `repo`. Default `file` if unclear.
3. Output counts per context level.

**Output format**:
```json
{
  "by_context_level": {
    "diff": 10,
    "file": 3,
    "repo": 2
  },
  "limitation": "Heuristic; actual context level may differ"
}
```

**Pass/fail**: Report-only.

---

### A10: RAG Ablation

**Purpose**: Compare runs with RAG (symbol definitions) enabled vs disabled. Measures impact of RAG on finding count and categories.

**Prerequisites**: Repo with code that has symbols (e.g., Go, TypeScript); Ollama running.

**Inputs**:
- `repo_root`, `ref`, `model`.

**Algorithm**:
1. Run A: `stet start ref --json --rag-symbol-max-definitions=0` (or `STET_RAG_SYMBOL_MAX_DEFINITIONS=0`). Save findings to `findings_no_rag.json`; `stet finish`.
2. Run B: `stet start ref --json` (default RAG, 10 definitions). Save findings to `findings_with_rag.json`.
3. Compare: count difference; category distribution difference; optional overlap analysis.

**Output format**:
```json
{
  "without_rag_count": 8,
  "with_rag_count": 10,
  "category_diff": {
    "correctness": {"without": 2, "with": 4},
    "maintainability": {"without": 5, "with": 5}
  }
}
```

**Pass/fail**: Report-only.

---

## Part 2: Human Tests

Each human test includes experiment design, protocol, recording instructions, and analysis steps.

---

### H1: Blind Triage Study

**Purpose**: Measure precision (share of findings that are actionable) per model, with model identity hidden to reduce bias.

**Prerequisites**: Findings from two or more models (or runs) on the same diffs; ability to shuffle and anonymize.

**Experiment design**:
- Select 50–100 findings per model from runs on the same baseline..HEAD.
- Shuffle all findings; remove model identifier; assign each a random ID (e.g., F001, F002).
- Single human (or multiple; compute inter-rater agreement if so) labels each finding.

**Step-by-step protocol**:
1. Export findings from each model run to CSV: `id`, `file`, `line`, `message`, `suggestion`, `severity`, `category`.
2. Combine CSVs; add column `anon_id`; remove model column; shuffle rows.
3. For each finding, open the file at the line and read the code context.
4. Label: `actionable` | `false_positive` | `wrong_suggestion` | `out_of_scope`. Add optional `notes`.
5. Record labels in a spreadsheet with `anon_id` and label.
6. After all labels collected, map `anon_id` back to model (using a separate key file).

**What to record**: `anon_id`, `label`, `notes`, `time_spent_seconds` (optional).

**Sample size guidance**: 50–100 findings per model for a meaningful precision estimate. Fewer if only comparing two models; more if confidence intervals are desired.

**Analysis**:
- Precision per model = (count labeled `actionable`) / (total labeled).
- Per-reason breakdown: count of `false_positive`, `wrong_suggestion`, `out_of_scope`.
- If multiple raters: compute agreement (e.g., Cohen's kappa) before aggregating.

**Artifacts**: CSV with `anon_id`, `label`, `notes`; summary report with precision per model.

---

### H2: Fixture Benchmark

**Purpose**: Create ground truth for precision, recall, and F1. Diffs with known issues; human labels which issues exist; compare Stet output to labels.

**Prerequisites**: Ability to create or select diffs with known defects (bugs, style issues, security issues).

**Experiment design**:
- Create 10–30 diffs (or select from real PRs) where you know the true positives: "This diff introduces bug X at file:line" or "This diff has style issue Y."
- For each diff, produce a ground-truth JSON: `[{"file": "...", "line": N, "issue_type": "bug"|"style"|..., "description": "..."}]`.
- Run Stet on each diff; human matches Stet findings to ground truth (TP/FP/FN).

**Step-by-step protocol**:
1. Create or select diff; document expected issues in ground-truth JSON.
2. Run `stet start ref --json` (or equivalent for that diff); save findings.
3. For each Stet finding: is it a TP (matches a ground-truth issue), FP (does not match), or is it a new valid issue? If new valid issue, add to ground truth and treat as TP.
4. For each ground-truth issue: was it found by Stet? If not, FN.
5. Record TP, FP, FN per diff; aggregate.

**What to record**: Per diff: `diff_id`, `TP`, `FP`, `FN`; optionally per-finding match details.

**Sample size guidance**: 10–30 diffs; 2–10 issues per diff. Balance coverage of categories (security, correctness, style).

**Analysis**:
- Precision = TP / (TP + FP); Recall = TP / (TP + FN); F1 = 2 * P * R / (P + R).
- Report per diff and aggregate; optionally per category.

**Artifacts**: Ground-truth JSON files; findings JSON per diff; summary with P, R, F1.

---

### H3: Self-Review Dogfood

**Purpose**: Use Stet on the Stet repo; triage findings with `stet dismiss`; derive actionability from history. Real project, real usage.

**Prerequisites**: Stet repo; Ollama with model; familiarity with the codebase.

**Experiment design**:
- Run Stet on recent commits (e.g., `stet start HEAD~5`).
- Triage every finding: either fix the issue or `stet dismiss <id> <reason>`.
- Finish session; analyze history.

**Step-by-step protocol**:
1. `stet start HEAD~5` (or chosen ref).
2. For each finding: read code; decide: fix in code and commit, or `stet dismiss <id> <reason>` with one of `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`.
3. Re-run `stet run` after fixes; repeat until all findings triaged.
4. `stet finish`.
5. Run automated test A2 on `.review/history.jsonl` to get actionability and reason breakdown.
6. Optionally: add recurring patterns to [review-quality.md](review-quality.md) curated false-positive table.

**What to record**: Dismissal reasons; notes on any new false-positive patterns.

**Sample size guidance**: One full review session; aim for 20+ findings to get meaningful actionability.

**Analysis**: Actionability rate; per-reason counts; qualitative notes on patterns.

**Artifacts**: Updated `history.jsonl`; optional updates to `review-quality.md`.

---

### H4: Cross-Tool Comparison

**Purpose**: Compare Stet to another LLM-powered review tool (e.g., RoboRev, Graphite) on the same diffs. Measure overlap and unique value per tool.

**Prerequisites**: Stet and at least one other tool; same diffs run through both; ability to normalize outputs (file, line, message).

**Experiment design**:
- Select 5–15 diffs with non-trivial changes.
- Run Stet; export findings.
- Run other tool on same diffs; export findings.
- Normalize to common schema (file, line, message).
- Human labels: for overlap (both tools found similar issue) and unique (only one tool found it); label unique findings as valid or invalid.

**Step-by-step protocol**:
1. For each diff: run Stet, save findings; run other tool, save findings.
2. Normalize outputs to (file, line, message) or equivalent.
3. Match findings across tools: same file:line and similar message → overlap.
4. For unique findings (only Stet or only other tool): human labels valid/invalid.
5. Compute: overlap count; unique-Stet count (and how many valid); unique-other count (and how many valid).

**What to record**: Per finding: tool, file, line, message, overlap_with (other finding id or none), unique_valid (yes/no).

**Sample size guidance**: 5–15 diffs; 5–30 findings per tool per diff.

**Analysis**: Overlap rate; precision of unique findings per tool; qualitative comparison.

**Artifacts**: Normalized findings CSV; overlap matrix; summary report.

---

### H5: LLM-as-Judge Calibration

**Purpose**: Check if an LLM can reliably label findings (actionable vs not) in agreement with humans. If yes, LLM-as-judge can scale human evaluation.

**Prerequisites**: Subset of findings with human labels (e.g., from H1); access to an LLM API (e.g., Claude, GPT) for judging.

**Experiment design**:
- Take 50–100 findings with human labels from H1 (or similar).
- Send each finding (file, line, message, code snippet) to judge LLM with prompt: "Is this code review finding actionable? Respond: actionable | false_positive | wrong_suggestion | out_of_scope."
- Compare judge labels to human labels.

**Step-by-step protocol**:
1. Export human-labeled findings with labels.
2. For each finding: construct prompt with file, line, message, and code context (e.g., 5 lines before/after).
3. Call judge LLM; parse response into one of the four labels.
4. Record judge label and human label.
5. Compute agreement: exact match rate; optionally Cohen's kappa.

**What to record**: `finding_id`, `human_label`, `judge_label`, `match` (boolean).

**Sample size guidance**: 50–100 findings. If agreement < 85%, do not use judge alone for evaluation.

**Analysis**: Agreement rate; confusion matrix (human vs judge); per-reason accuracy.

**Artifacts**: Comparison CSV; agreement report.

---

### H6: Curated False-Positive Audit

**Purpose**: Identify new false-positive patterns from Stet runs and add them to the curated table in [review-quality.md](review-quality.md) for prompt shadowing and optimizer.

**Prerequisites**: Stet runs that produced findings; access to review-quality.md.

**Experiment design**:
- Run Stet on one or more repos; collect findings that were dismissed as false_positive or wrong_suggestion.
- Cluster by message pattern or category; identify recurring patterns.
- Add new patterns to the curated table with category, message_pattern, reason, note.

**Step-by-step protocol**:
1. Run Stet; triage findings; record dismissals with reasons.
2. Filter to `false_positive` and `wrong_suggestion`.
3. Group by similar message (e.g., substring or keyword).
4. For each group: decide if it merits a curated entry. If yes, add to review-quality.md table: category, message_pattern, reason, note.
5. Follow schema in review-quality.md (see "Known false positive patterns" and "Schema for false positive entries").

**What to record**: Pattern, reason, example finding, note.

**Sample size guidance**: Continue until no new patterns emerge from last N sessions (e.g., 5–10).

**Analysis**: Count of new patterns added; optional reduction in similar future false positives.

**Artifacts**: Updated review-quality.md.

---

### H7: Suggestion Quality Assessment

**Purpose**: Score the quality of suggested fixes: correct/safe, partial, wrong, or harmful. Complements precision of the finding itself.

**Prerequisites**: Findings with `suggestion` field; human can evaluate code changes.

**Experiment design**:
- Sample 30–50 findings that have a non-empty suggestion.
- Human evaluates each suggestion in context: would applying it fix the issue correctly, partially, or make things worse?

**Step-by-step protocol**:
1. Export findings with `suggestion`; filter to non-empty.
2. For each: read file, line, message, suggestion.
3. Label: `correct_safe` | `partial` | `wrong` | `harmful`. Optionally add note.
4. Record in spreadsheet.

**What to record**: `finding_id`, `suggestion_quality`, `notes`.

**Sample size guidance**: 30–50 findings with suggestions.

**Analysis**: Distribution of quality; percentage correct_safe; percentage harmful (critical to minimize).

**Artifacts**: CSV; summary report.

---

### H8: Severity Calibration

**Purpose**: Check if Stet's severity (error, warning, info, nitpick) matches human expectation. High misclassification erodes trust.

**Prerequisites**: Sample of findings; human can judge appropriate severity.

**Experiment design**:
- Take 30–50 findings across severities.
- Human labels: for each, is the assigned severity correct, or should it be higher/lower?

**Step-by-step protocol**:
1. Export findings with severity.
2. For each: read finding and code context.
3. Label: `correct` | `too_high` | `too_low`. Optionally suggest correct severity.
4. Record.

**What to record**: `finding_id`, `assigned_severity`, `human_verdict`, `suggested_severity` (optional).

**Sample size guidance**: 30–50 findings; strive for mix of severities.

**Analysis**: Misclassification rate (too_high + too_low); confusion matrix.

**Artifacts**: CSV; summary report.

---

### H9: User Preference A/B

**Purpose**: Compare models by real-world usage: satisfaction, time-to-triage, perceived usefulness over days of use.

**Prerequisites**: Two models to compare; developer(s) willing to use each for a period.

**Experiment design**:
- Use model A for N days (e.g., 5–7); use model B for N days. Counterbalance order (half use A first, half B first if multiple users).
- Track: time spent triaging per session; number of actionable fixes applied; qualitative preference.

**Step-by-step protocol**:
1. Define period length (e.g., 5 days per model).
2. Use Stet with model A exclusively for period 1; record after each session: findings count, dismissals, fixes applied, time spent (minutes).
3. Switch to model B for period 2; same recording.
4. Survey: which model did you prefer? Why? What was different?
5. Aggregate metrics; compare.

**What to record**: Per session: model, findings_count, dismissals_count, fixes_applied, time_minutes. Final: preference, free-form feedback.

**Sample size guidance**: At least 2–3 sessions per model per user; multiple users improve confidence.

**Analysis**: Mean time per session; mean actionable rate; preference count; qualitative themes.

**Artifacts**: Session log; survey responses; summary report.

---

## Quick Reference

| ID | Name | Type | Description |
|----|------|------|-------------|
| A1 | Same-diff model swap | Automated | Compare two models on same diff; counts, overlap, unique |
| A2 | Actionability from history | Automated | Parse history.jsonl; actionability rate, reason breakdown |
| A3 | Latency and throughput | Automated | Wall-clock time, hunks/sec |
| A4 | Repeatability | Automated | Same model N runs; Jaccard similarity |
| A5 | Category/severity distribution | Automated | Counts by category and severity |
| A6 | Finding-ID stability | Automated | Assert IDs stable across runs |
| A7 | Dry-run regression | Automated | Schema validation; CI without Ollama |
| A8 | Multi-run aggregation | Automated | Merge N runs; optional ground-truth precision/recall |
| A9 | Context-level tagging | Automated | Heuristic diff/file/repo tagging |
| A10 | RAG ablation | Automated | Compare RAG on vs off |
| H1 | Blind triage | Human | Label findings; precision per model |
| H2 | Fixture benchmark | Human | Ground truth; precision, recall, F1 |
| H3 | Self-review dogfood | Human | Triage on Stet repo; actionability from history |
| H4 | Cross-tool comparison | Human | Stet vs other tool; overlap, unique value |
| H5 | LLM-as-judge calibration | Human | Compare judge LLM to human labels |
| H6 | Curated FP audit | Human | Add patterns to review-quality.md |
| H7 | Suggestion quality | Human | Score suggestion correctness |
| H8 | Severity calibration | Human | Check severity matches expectation |
| H9 | User preference A/B | Human | Compare models over days of use |
