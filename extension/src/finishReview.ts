import { spawnStet } from "./cli";
import type { FindingsTreeDataProvider } from "./findingsPanel";

export interface FinishReviewResult {
  ok: boolean;
  stderr: string;
  exitCode: number;
}

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
