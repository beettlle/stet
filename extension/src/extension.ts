import * as vscode from "vscode";

import { spawnStet, spawnStetStream } from "./cli";
import { buildCopyForChatBlock } from "./copyForChat";
import { createFindingsPanel } from "./findingsPanel";
import { runFinishReview } from "./finishReview";
import type { TreeItemModel } from "./findingsPanel";
import { openFinding } from "./openFinding";
import type { OpenFindingPayload } from "./openFinding";
import type { Finding } from "./contract";
import { parseStreamEvent } from "./parse";

function getRepoRoot(): string | null {
  const folders = vscode.workspace.workspaceFolders;
  if (!folders || folders.length === 0) return null;
  return folders[0].uri.fsPath;
}

/**
 * Surfaces CLI error to the user (stderr + exit code meaning).
 */
function showCLIError(stderr: string, exitCode: number): void {
  const trimmed = stderr.trim();
  const detail = trimmed ? trimmed : `Exit code ${exitCode}`;
  if (exitCode === 2) {
    void vscode.window.showErrorMessage(
      `Stet: Ollama unreachable. ${detail}`,
      { modal: false }
    );
    return;
  }
  void vscode.window.showErrorMessage(`Stet: ${detail}`, { modal: false });
}

export function activate(context: vscode.ExtensionContext): void {
  const findingsProvider = createFindingsPanel(context);
  context.subscriptions.push(
    vscode.commands.registerCommand("stet.openFinding", async (payload: OpenFindingPayload) => {
      const root = getRepoRoot();
      if (!root) {
        void vscode.window.showErrorMessage(
          "Stet: No workspace folder open. Open a folder to open findings."
        );
        return;
      }
      await openFinding(payload, root);
    })
  );
  context.subscriptions.push(
    vscode.commands.registerCommand(
      "stet.copyFindingForChat",
      async (element: TreeItemModel | undefined) => {
        if (element?.kind !== "finding") {
          return;
        }
        const root = getRepoRoot();
        if (!root) {
          void vscode.window.showErrorMessage(
            "Stet: No workspace folder open. Open a folder to copy findings."
          );
          return;
        }
        const markdown = buildCopyForChatBlock(element.finding, root);
        await vscode.env.clipboard.writeText(markdown);
        void vscode.window.setStatusBarMessage("Stet: Copied to clipboard", 2000);
      }
    )
  );
  context.subscriptions.push(
    vscode.commands.registerCommand("stet.startReview", async () => {
      const cwd = getRepoRoot();
      if (!cwd) {
        void vscode.window.showErrorMessage(
          "Stet: No workspace folder open. Open a folder that is a Git repository."
        );
        return;
      }
      findingsProvider.setScanning(true);
      const accumulatedFindings: Finding[] = [];
      void vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: "Stet",
          cancellable: false,
        },
        async () => {
          const result = await spawnStetStream(
            ["start", "--dry-run", "--quiet", "--json", "--stream"],
            { cwd },
            {
              onLine(line) {
                try {
                  const ev = parseStreamEvent(line);
                  if (ev.type === "progress") {
                    // Keep scanning; panel already shows scanning state
                    return;
                  }
                  if (ev.type === "finding") {
                    accumulatedFindings.push(ev.data);
                    findingsProvider.setFindings([...accumulatedFindings]);
                    return;
                  }
                  if (ev.type === "done") {
                    findingsProvider.setScanning(false);
                  }
                } catch (e) {
                  findingsProvider.setFindings([]);
                  findingsProvider.setScanning(false);
                  const message = e instanceof Error ? e.message : String(e);
                  console.error("Stet: parse stream error:", message);
                  void vscode.window.showErrorMessage("Stet: Invalid stream output from CLI.");
                }
              },
              onClose(_exitCode, _stderr) {
                // Only reset scanning indicator; error display is handled
                // after the await so it runs exactly once.
                findingsProvider.setScanning(false);
              },
            }
          );
          if (result.exitCode !== 0) {
            findingsProvider.setFindings([]);
            showCLIError(result.stderr, result.exitCode);
          } else {
            void vscode.window.showInformationMessage(
              `Stet: Review complete. ${accumulatedFindings.length} finding(s).`
            );
          }
        }
      );
    })
  );
  context.subscriptions.push(
    vscode.commands.registerCommand("stet.finishReview", async () => {
      const cwd = getRepoRoot();
      if (!cwd) {
        void vscode.window.showErrorMessage(
          "Stet: No workspace folder open. Open a folder that is a Git repository."
        );
        return;
      }
      void vscode.window.withProgress(
        {
          location: vscode.ProgressLocation.Notification,
          title: "Finishing review…",
          cancellable: false,
        },
        async () => {
          const result = await runFinishReview(cwd, findingsProvider);
          if (result.ok) {
            void vscode.window.showInformationMessage("Stet: Review finished.");
          } else {
            showCLIError(result.stderr, result.exitCode);
          }
        }
      );
    })
  );
}

export function deactivate(): void {}
