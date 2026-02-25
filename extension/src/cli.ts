/**
 * Spawns the stet CLI and captures stdout, stderr, and exit code.
 * Must be run from repository root (see docs/cli-extension-contract.md).
 */

import { spawn } from "child_process";

const CLI_PATH_REQUIRED_MSG = "cliPath must be a non-empty string";

export interface SpawnResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

export interface SpawnStetStreamCallbacks {
  onLine?: (line: string) => void;
  onClose: (exitCode: number, stderr: string) => void;
}

/**
 * Spawns stet with the given args from cwd (repo root).
 * @param args - CLI args, e.g. ["start", "--dry-run"] or ["run"]
 * @param options.cwd - Working directory (must be repo root)
 * @param options.cliPath - Optional path to stet binary (default "stet")
 * @returns Promise resolving with { exitCode, stdout, stderr } after the process closes
 */
export function spawnStet(
  args: string[],
  options: { cwd: string; cliPath?: string }
): Promise<SpawnResult> {
  const { cwd, cliPath = "stet" } = options;
  if (typeof cliPath !== "string" || cliPath.trim() === "") {
    return Promise.reject(new Error(CLI_PATH_REQUIRED_MSG));
  }
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

/**
 * Spawns stet with streaming stdout: calls onLine for each complete line, then onClose when the process exits.
 * Use for CLI invocations with --stream so the extension can process NDJSON events incrementally.
 * @param args - CLI args, e.g. ["start", "--dry-run", "--quiet", "--json", "--stream"]
 * @param options.cwd - Working directory (must be repo root)
 * @param options.cliPath - Optional path to stet binary (default "stet")
 * @param callbacks.onLine - Called for each line of stdout (without trailing newline)
 * @param callbacks.onClose - Called when the process exits with (exitCode, stderr)
 * @returns Promise that resolves with { exitCode, stderr } when the process closes
 */
export function spawnStetStream(
  args: string[],
  options: { cwd: string; cliPath?: string },
  callbacks: SpawnStetStreamCallbacks
): Promise<{ exitCode: number; stderr: string }> {
  const { cwd, cliPath = "stet" } = options;
  if (typeof cliPath !== "string" || cliPath.trim() === "") {
    return Promise.reject(new Error(CLI_PATH_REQUIRED_MSG));
  }
  const { onLine, onClose } = callbacks;
  return new Promise((resolve) => {
    const chunksErr: Buffer[] = [];
    // Node.js streams guarantee that all "data" events are emitted before
    // "close", so stdoutBuffer is never accessed concurrently despite being
    // used in both the "data" and "close" handlers.
    let stdoutBuffer = "";
    let closed = false;
    const finish = (exitCode: number, stderr: string) => {
      if (closed) return;
      closed = true;
      onClose(exitCode, stderr);
      resolve({ exitCode, stderr });
    };
    const proc = spawn(cliPath, args, {
      cwd,
      shell: false,
      stdio: ["ignore", "pipe", "pipe"],
    });
    proc.stdout?.on("data", (chunk: Buffer) => {
      stdoutBuffer += chunk.toString("utf8");
      const lines = stdoutBuffer.split("\n");
      stdoutBuffer = lines.pop() ?? "";
      for (const line of lines) {
        onLine?.(line);
      }
    });
    proc.stderr?.on("data", (chunk: Buffer) => chunksErr.push(chunk));
    proc.on("close", (code: number | null, signal: NodeJS.Signals | null) => {
      const exitCode =
        code !== null && code !== undefined
          ? code
          : signal === "SIGKILL"
            ? 137
            : 1;
      const stderr = Buffer.concat(chunksErr).toString("utf8");
      if (stdoutBuffer.trim() !== "") {
        onLine?.(stdoutBuffer.trimEnd());
      }
      finish(exitCode, stderr);
    });
    proc.on("error", (err: Error) => {
      finish(1, err.message);
    });
  });
}
