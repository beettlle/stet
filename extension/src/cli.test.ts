import { spawn } from "child_process";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { spawnStet, spawnStetStream } from "./cli";

vi.mock("child_process", () => ({ spawn: vi.fn() }));

type CloseCb = (code: number | null, signal: NodeJS.Signals | null) => void;
type ErrorCb = (err: Error) => void;

function getMockCallback<T>(value: unknown, label: string): T {
  if (value === undefined || typeof value !== "function") {
    throw new Error(`${label} callback not found or not a function`);
  }
  return value as T;
}

describe("spawnStet", () => {
  const mockSpawn = vi.mocked(spawn);

  beforeEach(() => {
    mockSpawn.mockClear();
  });

  it("resolves with exitCode 0 and captured stdout/stderr on close", async () => {
    const mockProc = {
      stdout: { on: vi.fn() },
      stderr: { on: vi.fn() },
      on: vi.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    const promise = spawnStet(["start", "--dry-run"], { cwd: "/repo" });

    expect(mockSpawn).toHaveBeenCalledWith("stet", ["start", "--dry-run"], {
      cwd: "/repo",
      shell: false,
      stdio: ["ignore", "pipe", "pipe"],
    });

    const closeCb = getMockCallback<CloseCb>(
      mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "close")?.[1],
      "close",
    );
    // Simulate stdout data (callbacks registered by spawnStet)
    for (const call of mockProc.stdout.on.mock.calls) {
      if (call[0] === "data") call[1](Buffer.from('{"findings":[]}\n'));
    }
    closeCb(0, null);

    const result = await promise;
    expect(result.exitCode).toBe(0);
    expect(result.stdout).toBe('{"findings":[]}\n');
    expect(result.stderr).toBe("");
  });

  it("uses custom cliPath when provided", async () => {
    const mockProc = {
      stdout: { on: vi.fn() },
      stderr: { on: vi.fn() },
      on: vi.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    spawnStet(["run"], { cwd: "/repo", cliPath: "/usr/bin/stet" });
    expect(mockSpawn).toHaveBeenCalledWith("/usr/bin/stet", ["run"], expect.any(Object));
  });

  it("resolves with non-zero exit code and stderr on close", async () => {
    const mockProc = {
      stdout: { on: vi.fn() },
      stderr: { on: vi.fn() },
      on: vi.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    const promise = spawnStet(["start"], { cwd: "/repo" });
    const closeCb = getMockCallback<CloseCb>(
      mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "close")?.[1],
      "close",
    );
    for (const call of mockProc.stderr.on.mock.calls) {
      if (call[0] === "data") call[1](Buffer.from("Not a git repository\n"));
    }
    closeCb(1, null);

    const result = await promise;
    expect(result.exitCode).toBe(1);
    expect(result.stderr).toBe("Not a git repository\n");
  });

  it("resolves with exitCode 137 when signal is SIGKILL", async () => {
    const mockProc = {
      stdout: { on: vi.fn() },
      stderr: { on: vi.fn() },
      on: vi.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    const promise = spawnStet(["start"], { cwd: "/repo" });
    const closeCb = getMockCallback<CloseCb>(
      mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "close")?.[1],
      "close",
    );
    closeCb(null, "SIGKILL");

    const result = await promise;
    expect(result.exitCode).toBe(137);
  });

  it("resolves with exitCode 1 and error message on spawn error", async () => {
    const mockProc = {
      stdout: { on: vi.fn() },
      stderr: { on: vi.fn() },
      on: vi.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    const promise = spawnStet(["start"], { cwd: "/repo" });
    const errCb = getMockCallback<ErrorCb>(
      mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "error")?.[1],
      "error",
    );
    errCb(new Error("ENOENT: stet not found"));

    const result = await promise;
    expect(result.exitCode).toBe(1);
    expect(result.stdout).toBe("");
    expect(result.stderr).toBe("ENOENT: stet not found");
  });
});

describe("spawnStetStream", () => {
  const mockSpawn = vi.mocked(spawn);

  beforeEach(() => {
    mockSpawn.mockClear();
  });

  it("calls onLine for each complete line and onClose with exitCode and stderr", async () => {
    const mockProc = {
      stdout: { on: vi.fn() },
      stderr: { on: vi.fn() },
      on: vi.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    const lines: string[] = [];
    const promise = spawnStetStream(
      ["start", "--dry-run", "--quiet", "--json", "--stream"],
      { cwd: "/repo" },
      {
        onLine(line) {
          lines.push(line);
        },
        onClose(exitCode, stderr) {
          expect(exitCode).toBe(0);
          expect(stderr).toBe("");

          expect(lines).toHaveLength(2);
          expect(lines[0]).toBe('{"type":"progress","msg":"1 hunks to review"}');
          expect(lines[1]).toBe('{"type":"done"}');
        },
      }
    );

    const dataCb = mockProc.stdout.on.mock.calls.find((c: [string, unknown]) => c[0] === "data")?.[1] as (chunk: Buffer) => void;
    if (!dataCb) throw new Error("data callback not found");
    dataCb(Buffer.from('{"type":"progress","msg":"1 hunks to review"}\n'));
    dataCb(Buffer.from('{"type":"done"}\n'));

    const closeCb = getMockCallback<CloseCb>(
      mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "close")?.[1],
      "close",
    );
    for (const call of mockProc.stderr.on.mock.calls) {
      if (call[0] === "data") (call[1] as (chunk: Buffer) => void)(Buffer.from(""));
    }
    closeCb(0, null);

    const result = await promise;
    expect(result.exitCode).toBe(0);
    expect(result.stderr).toBe("");
    expect(lines).toHaveLength(2);
  });

  it("flushes remaining buffer without newline on close", async () => {
    const mockProc = {
      stdout: { on: vi.fn() },
      stderr: { on: vi.fn() },
      on: vi.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    const lines: string[] = [];
    const promise = spawnStetStream(
      ["start", "--stream"],
      { cwd: "/repo" },
      {
        onLine(line) {
          lines.push(line);
        },
        onClose() {},
      }
    );

    const dataCb = mockProc.stdout.on.mock.calls.find((c: [string, unknown]) => c[0] === "data")?.[1] as (chunk: Buffer) => void;
    if (!dataCb) throw new Error("data callback not found");
    dataCb(Buffer.from('{"type":"done"}'));
    const closeCb = getMockCallback<CloseCb>(
      mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "close")?.[1],
      "close",
    );
    closeCb(0, null);

    await promise;
    expect(lines).toContain('{"type":"done"}');
  });

  it("calls onClose with non-zero exitCode and stderr on failure", async () => {
    const mockProc = {
      stdout: { on: vi.fn() },
      stderr: { on: vi.fn() },
      on: vi.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    let capturedExitCode: number | null = null;
    let capturedStderr: string | null = null;
    const promise = spawnStetStream(
      ["start", "--stream"],
      { cwd: "/repo" },
      {
        onClose(exitCode, stderr) {
          capturedExitCode = exitCode;
          capturedStderr = stderr;
        },
      }
    );

    const closeCb = getMockCallback<CloseCb>(
      mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "close")?.[1],
      "close",
    );
    for (const call of mockProc.stderr.on.mock.calls) {
      if (call[0] === "data") (call[1] as (chunk: Buffer) => void)(Buffer.from("No active session\n"));
    }
    closeCb(1, null);

    const result = await promise;
    expect(result.exitCode).toBe(1);
    expect(result.stderr).toBe("No active session\n");
    expect(capturedExitCode).toBe(1);
    expect(capturedStderr).toBe("No active session\n");
  });
});
