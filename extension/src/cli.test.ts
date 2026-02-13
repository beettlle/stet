import { spawn } from "child_process";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { spawnStet } from "./cli";

vi.mock("child_process", () => ({ spawn: vi.fn() }));

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

    const closeCb = mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "close")?.[1];
    expect(closeCb).toBeDefined();
    // Simulate stdout data (callbacks registered by spawnStet)
    for (const call of mockProc.stdout.on.mock.calls) {
      if (call[0] === "data") call[1](Buffer.from('{"findings":[]}\n'));
    }
    (closeCb as (code: number | null, signal: NodeJS.Signals | null) => void)(0, null);

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
    const closeCb = mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "close")?.[1];
    for (const call of mockProc.stderr.on.mock.calls) {
      if (call[0] === "data") call[1](Buffer.from("Not a git repository\n"));
    }
    (closeCb as (code: number | null, signal: NodeJS.Signals | null) => void)(1, null);

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
    const closeCb = mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "close")?.[1];
    (closeCb as (code: number | null, signal: NodeJS.Signals | null) => void)(null, "SIGKILL");

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
    const errCb = mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "error")?.[1];
    (errCb as (err: Error) => void)(new Error("ENOENT: stet not found"));

    const result = await promise;
    expect(result.exitCode).toBe(1);
    expect(result.stdout).toBe("");
    expect(result.stderr).toBe("ENOENT: stet not found");
  });
});
