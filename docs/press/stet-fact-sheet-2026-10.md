# Stet fact sheet (October 2026)

Stet is an open-source (MIT) AI code reviewer that runs on your own machine. By default it uses a local model through Ollama or any OpenAI-compatible local server, so code and prompts never leave the computer and there is no per-review API bill. It reviews only the lines you changed, and you decide what to fix.

All numbers below were measured on 2026-10-18 unless marked otherwise. Each one has a footnote with the command or file that produced it.

## Quote

> "Stet does code review on the computer you already own. On a Mac Studio it reads about 21 tokens a second, roughly 10 seconds of model time per change, with no API bill and no code leaving the machine. In our seeded-bug benchmark it caught 10 of 12 planted defects in every one of three runs, including all 8 correctness bugs, the SQL injection, and the path traversal. Across 365 review sessions on its own codebase it raised 23,775 findings, 4,638 of them flagged as bugs and 879 as security issues. A review that would cost about 10 cents on a frontier cloud model costs nothing extra here."

## Accuracy on planted bugs

| Metric | Value |
|---|---|
| Planted bugs caught in every run | 10 of 12 (83%) [1] |
| Correctness bugs caught | 8 of 8, every run [1] |
| Security bugs caught | 2 of 3 (SQL injection, path traversal) [1] |
| Performance bug (resource leak) | Found in every run, but on the line where `defer` was removed rather than the expected line, so strict scoring counts it as missed [1] |
| Runs completed without error | 52 of 52 (42 fixture runs, 10 real commits) [1] |
| Run-to-run agreement of findings | 89% overlap (Jaccard) [1] |
| Extra findings on the two bug-free cases | 7 across 6 runs [1] |

The benchmark has 14 small diffs in Go, Python, and TypeScript: 12 contain one planted bug each, and 2 are clean refactors. Stet ran each case 3 times with its verification pass (critic) on. The Oct 11 run with the critic off caught 12 of 12 [2]. The critic dropped the inverted admin-check finding that the model had produced, which is a known trade-off and an active area of tuning.

## Precision on real commits (provisional)

| Metric | Value |
|---|---|
| Real stet commits reviewed | 10, 84 changed hunks [1] |
| Findings produced | 49 (about 5 per commit) [1] |
| Findings judged actionable | 4 of 49 (8%), up from 0 of 32 on Oct 11 [3] |
| Actionable findings confirmed by later fixes | 2 (fixed in commits `cb3756f` and `5519845`) [3] |

These labels come from a second-rater pass. The author's own blind labels are pending (see Methodology). On real commits, most findings are still false positives, so precision is the main area of active work. The newer, smaller commits drew about 2 findings each, compared with 11 per commit on the larger SP-039 commits.

## Speed

| Metric | Value |
|---|---|
| Hardware | Mac Studio, Apple M1 Ultra, 64 GB [4] |
| Model | Qwen 3.8 27B (local, LM Studio) [4] |
| Throughput | 20.6–22.1 tokens/s end to end [5] |
| Model time per typical change hunk | About 9–10 s (3k-token prompt, 200-token reply) [5] |
| Median wall time per planted-bug case, critic on | 12–14 s on an idle machine (runs 2 and 3) [1] |

## Cost and energy

| Metric | Value |
|---|---|
| API fees on the default local setup | $0 |
| Tokens in 16 recorded self-review sessions | 585,290 prompt, 39,884 completion [6] |
| Same work priced at cloud list rates | $1.57 (Claude Sonnet 5.5), $1.65 (GPT-5.6 Terra), $3.14 (Claude Opus 5.5) [6] |
| Cloud cost per review avoided | About $0.10 (Sonnet 5.5) to $0.20 (Opus 5.5) [6] |
| Local energy for those 16 sessions | 6–21 Wh, assuming 30–100 W draw [6] |

## Engineering

| Metric | Value |
|---|---|
| Development history | 596 commits since 2026-02-10 [7] |
| Public releases | v1.0.0 (2026-10-14), v1.1.0 (2026-10-17) [8] |
| Release binaries | 6: macOS, Linux, Windows on amd64 and arm64, checksum-verified installer [8] |
| Go test functions | 1,048, plus 93 extension tests [7] |
| Go statement coverage | 83.9% [9] |
| Coverage gate in CI | 77% for the whole project, 72% for every file |

## Dogfooding (stet reviewing itself)

| Metric | Value |
|---|---|
| Review sessions | 365 [10] |
| Findings raised | 23,775 [10] |
| Findings by category | documentation 6,033; testing 5,200; bug 4,638; maintainability 4,572; performance 1,509; design 886; security 879; style 58 [10] |
| Findings explicitly dismissed as `false_positive` | 134 [10] |
| Diff reviewed in sessions with a stet git note | 546 hunks, about 1.24 million characters [6] |

Category counts are what the model labeled each finding, not confirmed defects. The 134 explicit `false_positive` dismissals are a lower bound: most findings were never formally triaged.

## Do not quote

`stet stats quality` also prints "acceptance rate" (0.96), "actionability" (0.13), and "clean commit rate" (0.00). Their names do not match what they compute. Acceptance rate is simply findings not dismissed, and actionability is the share of dismissals marked `already_correct`. Leave them out.

## Methodology and sources

All eval artifacts are in `docs/research/model-eval-2026-10/`.

1. `summary-agent-labels.json`, from `python3 scripts/model_eval.py score --cases scripts/model-eval-cases --results docs/research/model-eval-2026-10/results`. Runs used `run --critic` with stet built from `45b9393`, temperature 0.1, a 64k context, and the OpenJev System One critic. The pre-registered setup and commit list are in `preflight.txt`. The first of three fixture runs overlapped with an unrelated pi-spine batch using the same model server, so timings from that run are inflated.
2. `docs/research/model-eval/summary.json` (2026-10-11, critic off, same fixtures and model).
3. `blind-labels-agent.tsv`. The 49 real-commit findings were exported anonymized (`export-blind`) and labeled by an AI second rater. Each label was checked against the code at that commit, including `go vet` for every claimed compile error. The author labels the same file blind for the official number (`blind.tsv`); then `score --labels blind.tsv --blind-key blind-key.json`.
4. `preflight.txt` (`system_profiler SPHardwareDataType`, `lms ps`).
5. `benchmark.txt` (`stet benchmark --warmup`, two runs, model idle).
6. `stats.txt` (`stet stats energy` and `stet stats volume` over the full history). Cloud list prices per million input/output tokens were checked on 2026-10-18: Claude Sonnet 5.5 $2/$10 and Claude Opus 5.5 $4/$20 (platform.claude.com pricing), GPT-5.6 Terra $2/$12 (developers.openai.com pricing). Per-review cost is the total divided by 16 sessions. Energy is model time (744.8 s) times the assumed power draw.
7. `git rev-list --count HEAD`; `func Test` count under `cli/`; `it(`/`test(` count under `extension/src`.
8. GitHub releases for `beettlle/stet`.
9. `go tool cover -func=coverage.out` (coverage file generated 2026-10-18).
10. `stet stats quality` over `.review/history.jsonl`. This counts reruns of the same change, so sessions are not distinct commits.
