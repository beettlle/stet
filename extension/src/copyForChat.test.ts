import { describe, expect, it } from "vitest";
import type { Finding } from "./contract";
import { buildCopyForChatBlock } from "./copyForChat";

const WORKSPACE_ROOT = "/workspace/repo";

describe("buildCopyForChatBlock", () => {
  it("produces link with file:line and file URI with #L fragment for single line", () => {
    const finding: Finding = {
      file: "src/foo.ts",
      line: 10,
      severity: "warning",
      category: "style",
      confidence: 1.0,
      message: "Use const",
    };
    const out = buildCopyForChatBlock(finding, WORKSPACE_ROOT);
    expect(out).toContain("[src/foo.ts:10](");
    expect(out).toMatch(/\]\(file:\/\/.+#L10\)/);
    expect(out).toContain("> [!WARNING] Style");
    expect(out).toContain("> Use const");
  });

  it("uses range start-end in link text and L{start} in fragment when range is set", () => {
    const finding: Finding = {
      file: "pkg/main.go",
      line: 3,
      range: { start: 5, end: 7 },
      severity: "error",
      category: "bug",
      confidence: 0.9,
      message: "Possible nil dereference",
    };
    const out = buildCopyForChatBlock(finding, WORKSPACE_ROOT);
    expect(out).toContain("[pkg/main.go:5-7](");
    expect(out).toMatch(/\]\(file:\/\/.+#L5\)/);
    expect(out).toContain("> [!WARNING] Bug");
    expect(out).toContain("> Possible nil dereference");
  });

  it("maps error severity to WARNING admonition", () => {
    const finding: Finding = {
      file: "a.go",
      line: 1,
      severity: "error",
      category: "security",
      confidence: 1.0,
      message: "SQL injection risk",
    };
    const out = buildCopyForChatBlock(finding, WORKSPACE_ROOT);
    expect(out).toContain("> [!WARNING] Security");
  });

  it("maps info and nitpick to NOTE admonition", () => {
    const findingInfo: Finding = {
      file: "b.ts",
      line: 2,
      severity: "info",
      category: "maintainability",
      confidence: 0.8,
      message: "Consider adding a comment",
    };
    const outInfo = buildCopyForChatBlock(findingInfo, WORKSPACE_ROOT);
    expect(outInfo).toContain("> [!NOTE] Maintainability");

    const findingNitpick: Finding = {
      file: "c.js",
      line: 3,
      severity: "nitpick",
      category: "style",
      confidence: 0.5,
      message: "Trailing space",
    };
    const outNitpick = buildCopyForChatBlock(findingNitpick, WORKSPACE_ROOT);
    expect(outNitpick).toContain("> [!NOTE] Style");
  });

  it("prefixes each message line with blockquote when message has newlines", () => {
    const finding: Finding = {
      file: "src/file.ts",
      line: 10,
      severity: "warning",
      category: "correctness",
      confidence: 1.0,
      message: "Line one.\nLine two.\nLine three.",
    };
    const out = buildCopyForChatBlock(finding, WORKSPACE_ROOT);
    expect(out).toContain("> Line one.");
    expect(out).toContain("> Line two.");
    expect(out).toContain("> Line three.");
  });

  it("falls back to line 1 when line and range are missing", () => {
    const finding: Finding = {
      file: "no-line.txt",
      severity: "info",
      category: "documentation",
      confidence: 0.5,
      message: "No line given",
    };
    const out = buildCopyForChatBlock(finding, WORKSPACE_ROOT);
    expect(out).toContain("[no-line.txt:1](");
    expect(out).toMatch(/\]\(file:\/\/.+#L1\)/);
    expect(out).toContain("> No line given");
  });

  it("capitalizes category and replaces underscores for title", () => {
    const finding: Finding = {
      file: "x.go",
      line: 1,
      severity: "warning",
      category: "best_practice",
      confidence: 1.0,
      message: "Prefer idiomatic code",
    };
    const out = buildCopyForChatBlock(finding, WORKSPACE_ROOT);
    expect(out).toContain("> [!WARNING] Best practice");
  });

  it("produces exactly three logical parts: link, header, message", () => {
    const finding: Finding = {
      file: "single/line.ts",
      line: 42,
      severity: "error",
      category: "correctness",
      confidence: 1.0,
      message: "Single line message.",
    };
    const out = buildCopyForChatBlock(finding, WORKSPACE_ROOT);
    const lines = out.split("\n");
    expect(lines.length).toBeGreaterThanOrEqual(3);
    expect(lines[0]).toMatch(/^\[.+\]\(file:\/\/.+#L\d+\)$/);
    expect(lines[1]).toMatch(/^> \[!(WARNING|NOTE)\] .+$/);
    expect(lines[2]).toMatch(/^> .+$/);
  });
});
