import { beforeEach, describe, expect, it, vi } from "vitest";
import { maybeAutoFinishAfterReview, runFinishReview } from "./finishReview";

const mockSpawnStet = vi.fn();
vi.mock("./cli", () => ({ spawnStet: (...args: unknown[]) => mockSpawnStet(...args) }));

describe("runFinishReview", () => {
  const mockProvider = {
    clear: vi.fn(),
  };

  beforeEach(() => {
    mockSpawnStet.mockClear();
    mockProvider.clear.mockClear();
  });

  it("calls stet finish and clears panel on success", async () => {
    mockSpawnStet.mockResolvedValue({
      exitCode: 0,
      stdout: "",
      stderr: "",
    });

    const result = await runFinishReview("/repo", mockProvider as never);

    expect(mockSpawnStet).toHaveBeenCalledWith(["finish"], { cwd: "/repo" });
    expect(mockProvider.clear).toHaveBeenCalledOnce();
    expect(result).toEqual({ ok: true, stderr: "", exitCode: 0 });
  });

  it("does not clear panel and returns ok false on CLI failure", async () => {
    mockSpawnStet.mockResolvedValue({
      exitCode: 1,
      stdout: "",
      stderr: "No active session\n",
    });

    const result = await runFinishReview("/repo", mockProvider as never);

    expect(mockSpawnStet).toHaveBeenCalledWith(["finish"], { cwd: "/repo" });
    expect(mockProvider.clear).not.toHaveBeenCalled();
    expect(result).toEqual({
      ok: false,
      stderr: "No active session\n",
      exitCode: 1,
    });
  });

  it("does not clear panel on exit code 2", async () => {
    mockSpawnStet.mockResolvedValue({
      exitCode: 2,
      stdout: "",
      stderr: "Ollama unreachable",
    });

    const result = await runFinishReview("/workspace", mockProvider as never);

    expect(mockProvider.clear).not.toHaveBeenCalled();
    expect(result).toEqual({
      ok: false,
      stderr: "Ollama unreachable",
      exitCode: 2,
    });
  });

  it("returns ok true when clear throws (CLI succeeded)", async () => {
    mockSpawnStet.mockResolvedValue({
      exitCode: 0,
      stdout: "",
      stderr: "",
    });
    mockProvider.clear.mockImplementationOnce(() => {
      throw new Error("Panel clear failed");
    });

    const result = await runFinishReview("/repo", mockProvider as never);

    expect(mockSpawnStet).toHaveBeenCalledWith(["finish"], { cwd: "/repo" });
    expect(mockProvider.clear).toHaveBeenCalledOnce();
    expect(result.ok).toBe(true);
    expect(result.exitCode).toBe(0);
  });
});

describe("maybeAutoFinishAfterReview", () => {
  const mockProvider = { clear: vi.fn() };

  beforeEach(() => {
    mockSpawnStet.mockClear();
    mockProvider.clear.mockClear();
  });

  it("returns skipped when auto-finish disabled", async () => {
    const outcome = await maybeAutoFinishAfterReview("/repo", mockProvider as never, {
      autoFinish: false,
      dryRun: false,
    });
    expect(outcome).toBe("skipped");
    expect(mockSpawnStet).not.toHaveBeenCalled();
  });

  it("returns skipped when dry-run", async () => {
    const outcome = await maybeAutoFinishAfterReview("/repo", mockProvider as never, {
      autoFinish: true,
      dryRun: true,
    });
    expect(outcome).toBe("skipped");
    expect(mockSpawnStet).not.toHaveBeenCalled();
  });

  it("clears panel when session already ended (CLI auto-finish)", async () => {
    mockSpawnStet.mockResolvedValueOnce({
      exitCode: 1,
      stdout: "",
      stderr: "No active session\n",
    });

    const outcome = await maybeAutoFinishAfterReview("/repo", mockProvider as never, {
      autoFinish: true,
      dryRun: false,
    });

    expect(mockSpawnStet).toHaveBeenCalledWith(["status"], { cwd: "/repo" });
    expect(mockProvider.clear).toHaveBeenCalledOnce();
    expect(outcome).toBe("session_cleared");
  });

  it("returns findings_remain when list is non-empty", async () => {
    mockSpawnStet
      .mockResolvedValueOnce({ exitCode: 0, stdout: "baseline: abc\n", stderr: "" })
      .mockResolvedValueOnce({
        exitCode: 0,
        stdout: "abc1234  main.go:1  WARNING  issue\n",
        stderr: "",
      });

    const outcome = await maybeAutoFinishAfterReview("/repo", mockProvider as never, {
      autoFinish: true,
      dryRun: false,
    });

    expect(outcome).toBe("findings_remain");
    expect(mockProvider.clear).not.toHaveBeenCalled();
  });

  it("calls finish when zero active findings", async () => {
    mockSpawnStet
      .mockResolvedValueOnce({ exitCode: 0, stdout: "baseline: abc\n", stderr: "" })
      .mockResolvedValueOnce({ exitCode: 0, stdout: "", stderr: "" })
      .mockResolvedValueOnce({ exitCode: 0, stdout: "", stderr: "" });

    const outcome = await maybeAutoFinishAfterReview("/repo", mockProvider as never, {
      autoFinish: true,
      dryRun: false,
    });

    expect(mockSpawnStet).toHaveBeenNthCalledWith(3, ["finish"], { cwd: "/repo" });
    expect(mockProvider.clear).toHaveBeenCalledOnce();
    expect(outcome).toBe("finished");
  });
});
