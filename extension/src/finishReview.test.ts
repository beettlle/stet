import { beforeEach, describe, expect, it, vi } from "vitest";
import { runFinishReview } from "./finishReview";

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
