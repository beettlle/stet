import { beforeEach, describe, expect, it, vi } from "vitest";
import * as path from "path";
import { rangeFromFragment, openFinding } from "./openFinding";

const mockShowTextDocument = vi.fn();
const mockUriFile = vi.fn();
const mockUriParse = vi.fn();

vi.mock("vscode", () => {
  class Range {
    public start: { line: number; character: number };
    public end: { line: number; character: number };
    constructor(
      startLine: number,
      startCharacter: number,
      endLine: number,
      endCharacter: number
    ) {
      this.start = { line: startLine, character: startCharacter };
      this.end = { line: endLine, character: endCharacter };
    }
  }
  return {
    window: {
      showTextDocument: (...args: unknown[]) => mockShowTextDocument(...args),
    },
    Uri: {
      file: (p: string) => mockUriFile(p),
      parse: (uri: string, strict?: boolean) => mockUriParse(uri, strict),
    },
    Range,
  };
});

beforeEach(() => {
  mockShowTextDocument.mockReset();
  mockUriFile.mockReset();
  mockUriParse.mockReset();
});

describe("rangeFromFragment", () => {
  it("returns single-line range for L10", () => {
    const r = rangeFromFragment("L10");
    expect(r).toBeDefined();
    expect(r!.start.line).toBe(9);
    expect(r!.start.character).toBe(0);
    expect(r!.end.line).toBe(9);
    expect(r!.end.character).toBe(0);
  });

  it("returns multi-line range for L10-L12", () => {
    const r = rangeFromFragment("L10-L12");
    expect(r).toBeDefined();
    expect(r!.start.line).toBe(9);
    expect(r!.end.line).toBe(11);
  });

  it("accepts L10-L12 without second L", () => {
    const r = rangeFromFragment("L10-12");
    expect(r).toBeDefined();
    expect(r!.start.line).toBe(9);
    expect(r!.end.line).toBe(11);
  });

  it("returns undefined for empty or invalid fragment", () => {
    expect(rangeFromFragment("")).toBeUndefined();
    expect(rangeFromFragment("  ")).toBeUndefined();
    expect(rangeFromFragment("x")).toBeUndefined();
    expect(rangeFromFragment("L")).toBeUndefined();
  });

  it("returns undefined for negative line", () => {
    expect(rangeFromFragment("L0")).toBeUndefined();
  });
});

describe("openFinding", () => {
  const workspaceRoot = "/repo";

  it("uses file + line when no cursor_uri", async () => {
    const fakeUri = { fsPath: path.join(workspaceRoot, "src/a.ts") };
    mockUriFile.mockReturnValue(fakeUri);
    mockShowTextDocument.mockResolvedValue({});

    await openFinding(
      { file: "src/a.ts", line: 5 },
      workspaceRoot
    );

    expect(mockUriFile).toHaveBeenCalledWith(path.join(workspaceRoot, "src/a.ts"));
    expect(mockShowTextDocument).toHaveBeenCalledWith(
      fakeUri,
      expect.objectContaining({
        preview: false,
      })
    );
    const opts = mockShowTextDocument.mock.calls[0][1];
    expect(opts.selection).toBeDefined();
    expect(opts.selection.start.line).toBe(4);
    expect(opts.selection.end.line).toBe(4);
  });

  it("uses file + range when no cursor_uri", async () => {
    const fakeUri = { fsPath: path.join(workspaceRoot, "pkg/main.go") };
    mockUriFile.mockReturnValue(fakeUri);
    mockShowTextDocument.mockResolvedValue({});

    await openFinding(
      { file: "pkg/main.go", range: { start: 5, end: 7 } },
      workspaceRoot
    );

    expect(mockUriFile).toHaveBeenCalledWith(path.join(workspaceRoot, "pkg/main.go"));
    const call = mockShowTextDocument.mock.calls[0];
    expect(call[1].selection).toBeDefined();
    expect(call[1].selection.start.line).toBe(4);
    expect(call[1].selection.end.line).toBe(6);
  });

  it("uses cursor_uri when present and parseable", async () => {
    const strippedUri = {
      scheme: "file",
      fsPath: "/abs/path/main.go",
      fragment: "",
    };
    const parsedUri = {
      scheme: "file",
      fsPath: "/abs/path/main.go",
      fragment: "L10",
      with: (_change: Record<string, string>) => strippedUri,
    };
    mockUriParse.mockReturnValue(parsedUri);
    mockShowTextDocument.mockResolvedValue({});

    await openFinding(
      {
        file: "pkg/main.go",
        line: 5,
        cursor_uri: "file:///abs/path/main.go#L10",
      },
      workspaceRoot
    );

    expect(mockUriParse).toHaveBeenCalledWith("file:///abs/path/main.go#L10", true);
    expect(mockShowTextDocument).toHaveBeenCalledWith(
      strippedUri,
      expect.objectContaining({
        preview: false,
      })
    );
    const call = mockShowTextDocument.mock.calls[0];
    expect(call[1].selection).toBeDefined();
    expect(call[1].selection.start.line).toBe(9);
  });

  it("falls back to file+line when cursor_uri parse throws", async () => {
    mockUriParse.mockImplementation(() => {
      throw new Error("parse error");
    });
    const fakeUri = { fsPath: path.join(workspaceRoot, "a.ts") };
    mockUriFile.mockReturnValue(fakeUri);
    mockShowTextDocument.mockResolvedValue({});

    await openFinding(
      { file: "a.ts", line: 1, cursor_uri: "invalid" },
      workspaceRoot
    );

    expect(mockUriFile).toHaveBeenCalledWith(path.join(workspaceRoot, "a.ts"));
    expect(mockShowTextDocument).toHaveBeenCalledWith(
      fakeUri,
      expect.any(Object)
    );
  });
});
