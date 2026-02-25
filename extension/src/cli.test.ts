import { spawn } from "child_process";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { spawnStet, spawnStetStream } from "./cli";

vi.mock("child_process", () => ({ spawn: vi.fn() }));

type CloseCb = (code: number | null, signal: NodeJS.Signals | null) => void;
type ErrorCb = (err: Error) => void;

/** Extracts the single callback registered for `event` on a vi.fn() mock, asserting exactly one exists. */
function getEventCb<T>(onMock: ReturnType<typeof vi.fn>, event: string): T {
  const matches = onMock.mock.calls.filter((c: [string, unknown]) => c[0] === event);
  if (matches.length !== 1) {
    throw new Error(`Expected exactly 1 "${event}" handler, got ${matches.length}`);
  }
  const cb = matches[0][1];
  if (typeof cb !== "function") {
    throw new Error(`"${event}" handler is not a function`);
  }
  return cb as T;
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

    const closeCb = getEventCb<CloseCb>(mockProc.on, "close");
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

  it("rejects when cliPath is empty string and does not call spawn", async () => {
    const promise = spawnStet(["start"], { cwd: "/repo", cliPath: "" });
    await expect(promise).rejects.toThrow("cliPath must be a non-empty string");
    expect(mockSpawn).not.toHaveBeenCalled();
  });

  it("resolves with non-zero exit code and stderr on close", async () => {
    const mockProc = {
      stdout: { on: vi.fn() },
      stderr: { on: vi.fn() },
      on: vi.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    const promise = spawnStet(["start"], { cwd: "/repo" });
    const closeCb = getEventCb<CloseCb>(mockProc.on, "close");
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
    const closeCb = getEventCb<CloseCb>(mockProc.on, "close");
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
    const errCb = getEventCb<ErrorCb>(mockProc.on, "error");
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

    const dataCb = getEventCb<(chunk: Buffer) => void>(mockProc.stdout.on, "data");
    dataCb(Buffer.from('{"type":"progress","msg":"1 hunks to review"}\n'));
    dataCb(Buffer.from('{"type":"done"}\n'));

    const closeCb = getEventCb<CloseCb>(mockProc.on, "close");
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

    const dataCb = getEventCb<(chunk: Buffer) => void>(mockProc.stdout.on, "data");
    dataCb(Buffer.from('{"type":"done"}'));
    const closeCb = getEventCb<CloseCb>(mockProc.on, "close");
    closeCb(0, null);

    await promise;
    expect(lines).toContain('{"type":"done"}');
  });

  it("rejects when cliPath is empty string and does not call spawn", async () => {
    const promise = spawnStetStream(
      ["start", "--stream"],
      { cwd: "/repo", cliPath: "" },
      { onClose() {} }
    );
    await expect(promise).rejects.toThrow("cliPath must be a non-empty string");
    expect(mockSpawn).not.toHaveBeenCalled();
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

    const closeCb = getEventCb<CloseCb>(mockProc.on, "close");
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
