import { describe, expect, it } from "vitest";
import { parseFindingsJSON, parseFindingsNDJSON } from "./parse";

describe("parseFindingsJSON", () => {
  it("parses valid single-line JSON with empty findings", () => {
    const out = parseFindingsJSON('{"findings":[]}\n');
    expect(out.findings).toEqual([]);
  });

  it("parses valid single-line JSON with one finding", () => {
    const stdout = JSON.stringify({
      findings: [
        {
          id: "f1",
          file: "src/foo.ts",
          line: 10,
          severity: "warning",
          category: "style",
          confidence: 1.0,
          message: "Use const",
        },
      ],
    });
    const out = parseFindingsJSON(stdout);
    expect(out.findings).toHaveLength(1);
    expect(out.findings[0]).toMatchObject({
      id: "f1",
      file: "src/foo.ts",
      line: 10,
      severity: "warning",
      category: "style",
      confidence: 1.0,
      message: "Use const",
    });
  });

  it("parses valid JSON with all optional fields", () => {
    const stdout = JSON.stringify({
      findings: [
        {
          id: "f2",
          file: "pkg/main.go",
          line: 5,
          range: { start: 5, end: 7 },
          severity: "error",
          category: "bug",
          confidence: 0.9,
          message: "Possible nil dereference",
          suggestion: "Add nil check",
          cursor_uri: "file:///repo/pkg/main.go#L5",
        },
      ],
    });
    const out = parseFindingsJSON(stdout);
    expect(out.findings).toHaveLength(1);
    expect(out.findings[0].range).toEqual({ start: 5, end: 7 });
    expect(out.findings[0].suggestion).toBe("Add nil check");
    expect(out.findings[0].cursor_uri).toContain("main.go");
  });

  it("accepts trimmed stdout (leading/trailing whitespace)", () => {
    const out = parseFindingsJSON('  \n  {"findings":[]}  \n');
    expect(out.findings).toEqual([]);
  });

  it("throws on empty stdout", () => {
    expect(() => parseFindingsJSON("")).toThrow("Empty stdout");
    expect(() => parseFindingsJSON("   \n  ")).toThrow("Empty stdout");
  });

  it("throws on invalid JSON", () => {
    expect(() => parseFindingsJSON("not json")).toThrow("Invalid JSON");
    expect(() => parseFindingsJSON("{")).toThrow("Invalid JSON");
  });

  it("throws when root is not an object", () => {
    expect(() => parseFindingsJSON("[]")).toThrow("Expected JSON object");
    expect(() => parseFindingsJSON("null")).toThrow("Expected JSON object");
    expect(() => parseFindingsJSON("42")).toThrow("Expected JSON object");
  });

  it("throws when findings is missing or not array", () => {
    expect(() => parseFindingsJSON("{}")).toThrow("Missing or invalid 'findings' array");
    expect(() => parseFindingsJSON('{"findings":null}')).toThrow(
      "Missing or invalid 'findings' array"
    );
    expect(() => parseFindingsJSON('{"findings":{}}')).toThrow(
      "Missing or invalid 'findings' array"
    );
  });

  it("throws when a finding has invalid severity", () => {
    const stdout = JSON.stringify({
      findings: [{ file: "a.ts", severity: "critical", category: "bug", confidence: 1.0, message: "x" }],
    });
    expect(() => parseFindingsJSON(stdout)).toThrow("Invalid finding at index 0");
  });

  it("throws when a finding has invalid category", () => {
    const stdout = JSON.stringify({
      findings: [{ file: "a.ts", severity: "warning", category: "typo", confidence: 1.0, message: "x" }],
    });
    expect(() => parseFindingsJSON(stdout)).toThrow("Invalid finding at index 0");
  });

  it("throws when a finding is missing required confidence", () => {
    const stdout = JSON.stringify({
      findings: [{ file: "a.ts", severity: "warning", category: "style", message: "x" }],
    });
    expect(() => parseFindingsJSON(stdout)).toThrow("Invalid finding at index 0");
  });

  it("throws when a finding has confidence out of range", () => {
    const stdout = JSON.stringify({
      findings: [{ file: "a.ts", severity: "warning", category: "style", confidence: 1.5, message: "x" }],
    });
    expect(() => parseFindingsJSON(stdout)).toThrow("Invalid finding at index 0");
  });

  it("throws when a finding is missing required file", () => {
    const stdout = JSON.stringify({
      findings: [{ severity: "warning", category: "style", confidence: 1.0, message: "x" }],
    });
    expect(() => parseFindingsJSON(stdout)).toThrow("Invalid finding at index 0");
  });

  it("throws when a finding has empty file", () => {
    const stdout = JSON.stringify({
      findings: [{ file: "", severity: "warning", category: "style", confidence: 1.0, message: "x" }],
    });
    expect(() => parseFindingsJSON(stdout)).toThrow("Invalid finding at index 0");
  });

  it("throws when a finding has invalid range shape", () => {
    const stdout = JSON.stringify({
      findings: [
        {
          file: "a.ts",
          severity: "info",
          category: "style",
          confidence: 1.0,
          message: "x",
          range: { start: "1", end: 2 },
        },
      ],
    });
    expect(() => parseFindingsJSON(stdout)).toThrow("Invalid finding at index 0");
  });
});

describe("parseFindingsNDJSON", () => {
  it("parses single line (same as JSON)", () => {
    const out = parseFindingsNDJSON('{"findings":[{"file":"a.go","severity":"info","category":"style","confidence":1.0,"message":"m"}]}\n');
    expect(out).toHaveLength(1);
    expect(out[0].file).toBe("a.go");
    expect(out[0].message).toBe("m");
  });

  it("merges findings from multiple lines", () => {
    const stdout = [
      '{"findings":[{"file":"a.ts","severity":"warning","category":"bug","confidence":1.0,"message":"first"}]}',
      '{"findings":[{"file":"b.ts","severity":"error","category":"security","confidence":0.8,"message":"second"}]}',
    ].join("\n");
    const out = parseFindingsNDJSON(stdout);
    expect(out).toHaveLength(2);
    expect(out[0].file).toBe("a.ts");
    expect(out[0].message).toBe("first");
    expect(out[1].file).toBe("b.ts");
    expect(out[1].message).toBe("second");
  });

  it("returns empty array when no lines", () => {
    expect(parseFindingsNDJSON("")).toEqual([]);
    expect(parseFindingsNDJSON("\n  \n")).toEqual([]);
  });

  it("throws when a line is invalid JSON", () => {
    expect(() => parseFindingsNDJSON("not json\n")).toThrow("Line 1: Invalid JSON");
    expect(() =>
      parseFindingsNDJSON('{"findings":[]}\n{broken}\n')
    ).toThrow("Line 2: Invalid JSON");
  });

  it("throws when a line has no findings array", () => {
    expect(() => parseFindingsNDJSON('{"other":[]}\n')).toThrow(
      "Line 1: Missing or invalid 'findings' array"
    );
  });
});
