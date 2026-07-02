import { spawnStet } from "./cli";
import type { FindingsTreeDataProvider } from "./findingsPanel";

export interface FinishReviewResult {
  ok: boolean;
  stderr: string;
  exitCode: number;
}

export type AutoFinishOutcome =
  | "finished"
  | "session_cleared"
  | "skipped"
  | "findings_remain";

/**
 * Runs `stet finish` from the given cwd. On success clears the findings panel.
 * Caller should show success message or call showCLIError based on result.
 */
export async function runFinishReview(
  cwd: string,
  provider: FindingsTreeDataProvider
): Promise<FinishReviewResult> {
  const result = await spawnStet(["finish"], { cwd });
  if (result.exitCode === 0) {
    // Always attempt clear; if it throws, the CLI still succeeded so we
    // return ok: true. The panel may be stale, but the review is finished.
    try {
      provider.clear();
    } catch (e: unknown) {
      const err = e instanceof Error ? e : new Error(String(e));
      console.error("Failed to clear findings panel:", err.message, err.stack ?? "");
    }
    return { ok: true, stderr: result.stderr, exitCode: 0 };
  }
  return { ok: false, stderr: result.stderr, exitCode: result.exitCode };
}

function clearPanelSafe(provider: FindingsTreeDataProvider): void {
  try {
    provider.clear();
  } catch (e: unknown) {
    const err = e instanceof Error ? e : new Error(String(e));
    console.error("Failed to clear findings panel:", err.message, err.stack ?? "");
  }
}

/**
 * After a successful review, auto-finish when enabled and there are zero active findings.
 * Active findings exclude dismissed IDs (same semantics as CLI `stet list`).
 */
export async function maybeAutoFinishAfterReview(
  cwd: string,
  provider: FindingsTreeDataProvider,
  options: { autoFinish: boolean; dryRun: boolean }
): Promise<AutoFinishOutcome> {
  if (!options.autoFinish || options.dryRun) {
    return "skipped";
  }

  const status = await spawnStet(["status"], { cwd });
  if (status.exitCode !== 0) {
    // CLI already auto-finished (no active session).
    clearPanelSafe(provider);
    return "session_cleared";
  }

  const list = await spawnStet(["list"], { cwd });
  if (list.exitCode !== 0) {
    return "skipped";
  }
  if (list.stdout.trim().length > 0) {
    return "findings_remain";
  }

  const result = await runFinishReview(cwd, provider);
  return result.ok ? "finished" : "skipped";
}
