import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Finding } from "./contract";
import { FindingsTreeDataProvider, runDismissFinding } from "./findingsPanel";

const mockCreateTreeView = vi.fn();
const mockSpawnStet = vi.fn();

vi.mock("./cli", () => ({ spawnStet: (...args: unknown[]) => mockSpawnStet(...args) }));

vi.mock("vscode", () => {
  class EventEmitter {
    fire(): void {}
    get event() {
      return () => {};
    }
  }
  class TreeItem {
    label?: string;
    description?: string;
    tooltip?: unknown;
    command?: { command: string; title: string; arguments: unknown[] };
    iconPath?: unknown;
    collapsibleState?: number;
    constructor(label: string, collapsibleState?: number) {
      this.label = label;
      this.collapsibleState = collapsibleState;
    }
  }
  class ThemeIcon {
    constructor(public id: string) {}
  }
  class MarkdownString {
    value = "";
    appendMarkdown(s: string) {
      this.value += s;
    }
  }
  return {
    EventEmitter,
    TreeItem,
    ThemeIcon,
    MarkdownString,
    TreeItemCollapsibleState: { None: 0 },
    window: {
      createTreeView: (...args: unknown[]) => mockCreateTreeView(...args),
    },
  };
});

beforeEach(() => {
  mockCreateTreeView.mockClear();
  mockSpawnStet.mockClear();
});

const finding1: Finding = {
  id: "abc1234567890",
  file: "src/foo.ts",
  line: 10,
  severity: "warning",
  category: "style",
  confidence: 1.0,
  message: "Use const",
};
const finding2: Finding = {
  id: "def9876543210",
  file: "pkg/main.go",
  line: 5,
  range: { start: 5, end: 7 },
  severity: "error",
  category: "bug",
  confidence: 0.9,
  message: "Possible nil dereference",
  cursor_uri: "file:///repo/pkg/main.go#L5",
};

describe("FindingsTreeDataProvider", () => {
  it("returns one scanning node when scanning is true", () => {
    const provider = new FindingsTreeDataProvider();
    provider.setScanning(true);
    const children = provider.getChildren(undefined);
    expect(children).toHaveLength(1);
    expect(children[0]).toEqual({ kind: "scanning" });
  });

  it("returns empty when not scanning and no findings", () => {
    const provider = new FindingsTreeDataProvider();
    provider.setFindings([]);
    const children = provider.getChildren(undefined);
    expect(children).toEqual([]);
  });

  it("returns one node per finding when not scanning", () => {
    const provider = new FindingsTreeDataProvider();
    provider.setFindings([finding1, finding2]);
    const children = provider.getChildren(undefined);
    expect(children).toHaveLength(2);
    expect(children[0]).toEqual({ kind: "finding", finding: finding1 });
    expect(children[1]).toEqual({ kind: "finding", finding: finding2 });
  });

  it("setScanning(false) after setFindings shows findings", () => {
    const provider = new FindingsTreeDataProvider();
    provider.setFindings([finding1]);
    provider.setScanning(false);
    const children = provider.getChildren(undefined);
    expect(children).toHaveLength(1);
    expect(children[0]).toEqual({ kind: "finding", finding: finding1 });
  });

  it("getTreeItem for scanning returns item with Scanning label", () => {
    const provider = new FindingsTreeDataProvider();
    const item = provider.getTreeItem({ kind: "scanning" });
    expect(item.label).toBe("Scanning …");
    expect(item.command).toBeUndefined();
  });

  it("getTreeItem for finding returns item with label, description, command", () => {
    const provider = new FindingsTreeDataProvider();
    const item = provider.getTreeItem({ kind: "finding", finding: finding1 });
    expect(item.label).toBe("src/foo.ts:10");
    expect(item.description).toBe("warning · style");
    expect(item.contextValue).toBe("finding");
    expect(item.command).toEqual({
      command: "stet.openFinding",
      title: "Open at location",
      arguments: [
        {
          file: finding1.file,
          line: finding1.line,
          range: finding1.range,
          cursor_uri: finding1.cursor_uri,
        },
      ],
    });
  });

  it("getTreeItem for finding with range uses start-end in label", () => {
    const provider = new FindingsTreeDataProvider();
    const item = provider.getTreeItem({ kind: "finding", finding: finding2 });
    expect(item.label).toBe("pkg/main.go:5-7");
    expect(item.command!.arguments[0]).toEqual({
      file: "pkg/main.go",
      line: 5,
      range: { start: 5, end: 7 },
      cursor_uri: "file:///repo/pkg/main.go#L5",
    });
  });

  it("clear resets findings and scanning", () => {
    const provider = new FindingsTreeDataProvider();
    provider.setFindings([finding1]);
    provider.clear();
    const children = provider.getChildren(undefined);
    expect(children).toEqual([]);
  });

  it("removeFindingById removes matching finding", () => {
    const provider = new FindingsTreeDataProvider();
    provider.setFindings([finding1, finding2]);
    provider.removeFindingById(finding1.id!);
    const children = provider.getChildren(undefined);
    expect(children).toHaveLength(1);
    expect(children[0]).toEqual({ kind: "finding", finding: finding2 });
  });
});

describe("runDismissFinding", () => {
  beforeEach(() => {
    mockSpawnStet.mockClear();
  });

  it("calls stet dismiss without reason and removes finding on success", async () => {
    mockSpawnStet.mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });
    const provider = new FindingsTreeDataProvider();
    const finding: Finding = { ...finding1 };

    const result = await runDismissFinding("/repo", provider, finding);

    expect(mockSpawnStet).toHaveBeenCalledWith(["dismiss", "abc1234567890"], { cwd: "/repo" });
    expect(result.ok).toBe(true);
    expect(provider.getChildren(undefined)).toEqual([]);
  });

  it("calls stet dismiss with reason when provided", async () => {
    mockSpawnStet.mockResolvedValue({ exitCode: 0, stdout: "", stderr: "" });
    const provider = new FindingsTreeDataProvider();
    provider.setFindings([finding1]);

    const result = await runDismissFinding("/repo", provider, finding1, "false_positive");

    expect(mockSpawnStet).toHaveBeenCalledWith(
      ["dismiss", "abc1234567890", "false_positive"],
      { cwd: "/repo" }
    );
    expect(result.ok).toBe(true);
    expect(provider.getChildren(undefined)).toEqual([]);
  });

  it("does not remove finding when CLI fails", async () => {
    mockSpawnStet.mockResolvedValue({
      exitCode: 1,
      stdout: "",
      stderr: "No active session\n",
    });
    const provider = new FindingsTreeDataProvider();
    provider.setFindings([finding1]);

    const result = await runDismissFinding("/repo", provider, finding1);

    expect(result.ok).toBe(false);
    expect(result.stderr).toBe("No active session\n");
    expect(provider.getChildren(undefined)).toHaveLength(1);
  });

  it("returns error when finding has no id", async () => {
    const provider = new FindingsTreeDataProvider();
    const noId: Finding = {
      file: "a.ts",
      line: 1,
      severity: "warning",
      category: "style",
      confidence: 1,
      message: "msg",
    };

    const result = await runDismissFinding("/repo", provider, noId);

    expect(mockSpawnStet).not.toHaveBeenCalled();
    expect(result.ok).toBe(false);
    expect(result.stderr).toBe("Finding has no id");
  });
});
