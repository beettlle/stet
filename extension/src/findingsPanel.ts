import * as vscode from "vscode";

import { spawnStet } from "./cli";
import type { Category, Finding } from "./contract";

/** Display-only filters for the findings panel (session data unchanged). */
export interface FindingFilters {
  minConfidence: number;
  /** When set and non-empty, only these categories are shown. */
  categories?: Category[];
}

export const DEFAULT_FINDING_FILTERS: FindingFilters = {
  minConfidence: 0,
};

/** Returns true when a finding should appear in the panel. */
export function passesFindingFilters(
  finding: Finding,
  filters: FindingFilters
): boolean {
  if (finding.confidence < filters.minConfidence) {
    return false;
  }
  const allowlist = filters.categories;
  if (allowlist !== undefined && allowlist.length > 0) {
    return allowlist.includes(finding.category);
  }
  return true;
}

/** Count findings hidden by the active display filters. */
export function countHiddenByFilter(
  findings: Finding[],
  filters: FindingFilters
): number {
  return findings.filter((f) => !passesFindingFilters(f, filters)).length;
}

/** Dismissal reasons aligned with CLI (`stet dismiss <id> [reason]`). */
export const DISMISSAL_REASONS = [
  { label: "False positive", value: "false_positive" },
  { label: "Already correct", value: "already_correct" },
  { label: "Wrong suggestion", value: "wrong_suggestion" },
  { label: "Out of scope", value: "out_of_scope" },
] as const;

export type DismissalReason = (typeof DISMISSAL_REASONS)[number]["value"];

export interface DismissFindingResult {
  ok: boolean;
  stderr: string;
  exitCode: number;
}

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
  private filters: FindingFilters = { ...DEFAULT_FINDING_FILTERS };
  private treeView: vscode.TreeView<TreeItemModel> | undefined;

  attachTreeView(treeView: vscode.TreeView<TreeItemModel>): void {
    this.treeView = treeView;
    this.updateFilterMessage();
  }

  setFilters(filters: FindingFilters): void {
    this.filters = filters;
    this.updateFilterMessage();
    this._onDidChangeTreeData.fire();
  }

  getFilters(): FindingFilters {
    return this.filters;
  }

  getHiddenByFilterCount(): number {
    return countHiddenByFilter(this.findings, this.filters);
  }

  private updateFilterMessage(): void {
    if (this.treeView === undefined) {
      return;
    }
    const hidden = this.getHiddenByFilterCount();
    this.treeView.message =
      hidden > 0 ? `${hidden} hidden by filter` : undefined;
  }

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
    return this.findings
      .filter((finding) => passesFindingFilters(finding, this.filters))
      .map((finding) => ({ kind: "finding", finding }));
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
    this.updateFilterMessage();
    this._onDidChangeTreeData.fire();
  }

  removeFindingById(id: string): void {
    this.findings = this.findings.filter((f) => f.id !== id);
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
  const treeView = vscode.window.createTreeView(VIEW_ID, {
    treeDataProvider: provider,
  });
  provider.attachTreeView(treeView);
  return provider;
}

/**
 * Invokes `stet dismiss` for the finding and removes it from the active panel list on success.
 */
export async function runDismissFinding(
  cwd: string,
  provider: FindingsTreeDataProvider,
  finding: Finding,
  reason?: DismissalReason
): Promise<DismissFindingResult> {
  const id = finding.id;
  if (!id) {
    return { ok: false, stderr: "Finding has no id", exitCode: 1 };
  }
  const args = reason !== undefined ? ["dismiss", id, reason] : ["dismiss", id];
  const result = await spawnStet(args, { cwd });
  if (result.exitCode === 0) {
    provider.removeFindingById(id);
    return { ok: true, stderr: result.stderr, exitCode: 0 };
  }
  return { ok: false, stderr: result.stderr, exitCode: result.exitCode };
}
