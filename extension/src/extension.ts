import * as vscode from "vscode";

import { spawnStet } from "./cli";
import { parseFindingsJSON } from "./parse";

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
  context.subscriptions.push(
    vscode.commands.registerCommand("stet.startReview", async () => {
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
          title: "Stet",
          cancellable: false,
        },
        async () => {
          const result = await spawnStet(["start", "--dry-run"], { cwd });
          if (result.exitCode !== 0) {
            showCLIError(result.stderr, result.exitCode);
            return;
          }
          try {
            const { findings } = parseFindingsJSON(result.stdout);
            void vscode.window.showInformationMessage(
              `Stet: Review complete. ${findings.length} finding(s).`
            );
          } catch (e) {
            const message = e instanceof Error ? e.message : String(e);
            void vscode.window.showErrorMessage(`Stet: Failed to parse output. ${message}`);
          }
        }
      );
    })
  );
}

export function deactivate(): void {}
