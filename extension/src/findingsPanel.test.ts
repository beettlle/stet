import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Finding } from "./contract";
import { FindingsTreeDataProvider } from "./findingsPanel";

const mockCreateTreeView = vi.fn();

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
});

describe("FindingsTreeDataProvider", () => {
  const finding1: Finding = {
    file: "src/foo.ts",
    line: 10,
    severity: "warning",
    category: "style",
    message: "Use const",
  };
  const finding2: Finding = {
    file: "pkg/main.go",
    line: 5,
    range: { start: 5, end: 7 },
    severity: "error",
    category: "bug",
    message: "Possible nil dereference",
    cursor_uri: "file:///repo/pkg/main.go#L5",
  };

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
    expect(children[0]).toMatchObject({ kind: "finding", finding: finding1 });
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
    expect(item.command!.arguments[0]).toMatchObject({
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
});
