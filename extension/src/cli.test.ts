import { spawn } from "child_process";
import { spawnStet } from "./cli";

jest.mock("child_process");

describe("spawnStet", () => {
  const mockSpawn = spawn as jest.MockedFunction<typeof spawn>;

  beforeEach(() => {
    mockSpawn.mockReset();
  });

  it("resolves with exitCode 0 and captured stdout/stderr on close", async () => {
    const mockProc = {
      stdout: { on: jest.fn() },
      stderr: { on: jest.fn() },
      on: jest.fn(),
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
    closeCb(0, null);

    const result = await promise;
    expect(result.exitCode).toBe(0);
    expect(result.stdout).toBe('{"findings":[]}\n');
    expect(result.stderr).toBe("");
  });

  it("uses custom cliPath when provided", async () => {
    const mockProc = {
      stdout: { on: jest.fn() },
      stderr: { on: jest.fn() },
      on: jest.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    spawnStet(["run"], { cwd: "/repo", cliPath: "/usr/bin/stet" });
    expect(mockSpawn).toHaveBeenCalledWith("/usr/bin/stet", ["run"], expect.any(Object));
  });

  it("resolves with non-zero exit code and stderr on close", async () => {
    const mockProc = {
      stdout: { on: jest.fn() },
      stderr: { on: jest.fn() },
      on: jest.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    const promise = spawnStet(["start"], { cwd: "/repo" });
    const closeCb = mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "close")?.[1];
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
      stdout: { on: jest.fn() },
      stderr: { on: jest.fn() },
      on: jest.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    const promise = spawnStet(["start"], { cwd: "/repo" });
    const closeCb = mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "close")?.[1];
    closeCb(null, "SIGKILL");

    const result = await promise;
    expect(result.exitCode).toBe(137);
  });

  it("resolves with exitCode 1 and error message on spawn error", async () => {
    const mockProc = {
      stdout: { on: jest.fn() },
      stderr: { on: jest.fn() },
      on: jest.fn(),
    };
    mockSpawn.mockReturnValue(mockProc as never);

    const promise = spawnStet(["start"], { cwd: "/repo" });
    const errCb = mockProc.on.mock.calls.find((c: [string, unknown]) => c[0] === "error")?.[1];
    errCb(new Error("ENOENT: stet not found"));

    const result = await promise;
    expect(result.exitCode).toBe(1);
    expect(result.stdout).toBe("");
    expect(result.stderr).toBe("ENOENT: stet not found");
  });
});
