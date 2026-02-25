import * as vscode from "vscode";

import type { Finding } from "./contract";

/** Serializable payload for stet.openFinding command (file, line, range, cursor_uri). */
export interface OpenFindingPayload {
  file: string;
  line?: number;
  range?: { start: number; end: number };
  cursor_uri?: string;
}

export type TreeItemModel =
  | { kind: "scanning" }
  | { kind: "finding"; finding: Finding };

const SCANNING_LABEL = "Scanning …";

export class FindingsTreeDataProvider
  implements vscode.TreeDataProvider<TreeItemModel>
{
  private _onDidChangeTreeData = new vscode.EventEmitter<
    TreeItemModel | undefined | null | void
  >();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private findings: Finding[] = [];
  private scanning = false;

  getChildren(element?: TreeItemModel): TreeItemModel[] {
    if (element !== undefined) {
      return [];
    }
    if (this.scanning) {
      return [{ kind: "scanning" }];
    }
    if (this.findings.length === 0) {
      return [];
    }
    return this.findings.map((finding) => ({ kind: "finding", finding }));
  }

  getTreeItem(element: TreeItemModel): vscode.TreeItem {
    if (element.kind === "scanning") {
      const item = new vscode.TreeItem(SCANNING_LABEL);
      item.iconPath = new vscode.ThemeIcon("loading~spin");
      return item;
    }
    const { finding } = element;
    const linePart =
      finding.range !== undefined
        ? `:${finding.range.start}-${finding.range.end}`
        : finding.line !== undefined
          ? `:${finding.line}`
          : "";
    const label = `${finding.file}${linePart}`;
    const description = `${finding.severity} · ${finding.category}`;
    const tooltip = new vscode.MarkdownString();
    tooltip.appendMarkdown(`**${finding.file}${linePart}**\n\n`);
    tooltip.appendMarkdown(`${finding.severity} · ${finding.category}\n\n`);
    if (finding.message) {
      tooltip.appendMarkdown(finding.message);
    }

    const payload: OpenFindingPayload = {
      file: finding.file,
      line: finding.line,
      range: finding.range,
      cursor_uri: finding.cursor_uri,
    };
    const item = new vscode.TreeItem(label, vscode.TreeItemCollapsibleState.None);
    item.contextValue = "finding";
    item.description = description;
    item.tooltip = tooltip;
    item.command = {
      command: "stet.openFinding",
      title: "Open at location",
      arguments: [payload],
    };
    return item;
  }

  setScanning(scanning: boolean): void {
    this.scanning = scanning;
    this._onDidChangeTreeData.fire();
  }

  setFindings(findings: Finding[]): void {
    this.findings = findings;
    this.scanning = false;
    this._onDidChangeTreeData.fire();
  }

  clear(): void {
    this.findings = [];
    this.scanning = false;
    this._onDidChangeTreeData.fire();
  }
}

const VIEW_ID = "stetFindings";

/**
 * Creates the Findings panel (TreeView) and its data provider.
 * Call setScanning/setFindings/clear on the returned provider to update the view.
 */
export function createFindingsPanel(
  _context: vscode.ExtensionContext
): FindingsTreeDataProvider {
  const provider = new FindingsTreeDataProvider();
  void vscode.window.createTreeView(VIEW_ID, {
    treeDataProvider: provider,
  });
  return provider;
}
