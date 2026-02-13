/**
 * Spawns the stet CLI and captures stdout, stderr, and exit code.
 * Must be run from repository root (see docs/cli-extension-contract.md).
 */

import { spawn } from "child_process";

export interface SpawnResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

/**
 * Spawns stet with the given args from cwd (repo root).
 * @param args - CLI args, e.g. ["start", "--dry-run"] or ["run"]
 * @param options.cwd - Working directory (must be repo root)
 * @param options.cliPath - Optional path to stet binary (default "stet")
 */
export function spawnStet(
  args: string[],
  options: { cwd: string; cliPath?: string }
): Promise<SpawnResult> {
  const { cwd, cliPath = "stet" } = options;
  return new Promise((resolve) => {
    const chunksOut: Buffer[] = [];
    const chunksErr: Buffer[] = [];
    const proc = spawn(cliPath, args, {
      cwd,
      shell: false,
      stdio: ["ignore", "pipe", "pipe"],
    });
    proc.stdout?.on("data", (chunk: Buffer) => chunksOut.push(chunk));
    proc.stderr?.on("data", (chunk: Buffer) => chunksErr.push(chunk));
    proc.on("close", (code: number | null, signal: NodeJS.Signals | null) => {
      const exitCode =
        code !== null && code !== undefined
          ? code
          : signal === "SIGKILL"
            ? 137
            : 1;
      resolve({
        exitCode,
        stdout: Buffer.concat(chunksOut).toString("utf8"),
        stderr: Buffer.concat(chunksErr).toString("utf8"),
      });
    });
    proc.on("error", (err: Error) => {
      resolve({
        exitCode: 1,
        stdout: "",
        stderr: err.message,
      });
    });
  });
}
