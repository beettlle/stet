import * as vscode from "vscode";

export function activate(context: vscode.ExtensionContext): void {
  context.subscriptions.push(
    vscode.commands.registerCommand("stet.startReview", () => {
      void vscode.window.showInformationMessage("Stet: Start review (placeholder)");
    })
  );
}

export function deactivate(): void {}
