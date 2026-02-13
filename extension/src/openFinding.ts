import * as path from "path";
import * as vscode from "vscode";

/** Minimal finding payload for opening (serializable for command). */
export interface OpenFindingPayload {
  file: string;
  line?: number;
  range?: { start: number; end: number };
  cursor_uri?: string;
}

/**
 * Parses a URI fragment like "L10" or "L10-L12" into a VSCode Range (0-based lines).
 * Returns undefined if fragment is missing or invalid.
 */
export function rangeFromFragment(fragment: string): vscode.Range | undefined {
  if (!fragment || typeof fragment !== "string") return undefined;
  const trimmed = fragment.trim();
  const singleMatch = /^L(\d+)$/i.exec(trimmed);
  if (singleMatch) {
    const line = parseInt(singleMatch[1], 10) - 1;
    if (line < 0) return undefined;
    return new vscode.Range(line, 0, line, 0);
  }
  const rangeMatch = /^L(\d+)-L?(\d+)$/i.exec(trimmed);
  if (rangeMatch) {
    const startLine = parseInt(rangeMatch[1], 10) - 1;
    const endLine = parseInt(rangeMatch[2], 10) - 1;
    if (startLine < 0 || endLine < startLine) return undefined;
    return new vscode.Range(startLine, 0, endLine, 0);
  }
  return undefined;
}

/**
 * Opens the editor at the finding location. Uses cursor_uri when present and valid;
 * otherwise builds file URI from workspaceRoot + finding.file and uses line/range.
 */
export async function openFinding(
  finding: OpenFindingPayload,
  workspaceRoot: string
): Promise<vscode.TextEditor> {
  let uri: vscode.Uri;
  let selection: vscode.Range | undefined;

  if (finding.cursor_uri) {
    try {
      const parsed = vscode.Uri.parse(finding.cursor_uri, true);
      if (parsed.scheme === "file" || parsed.scheme === "cursor") {
        uri = parsed;
        selection = parsed.fragment
          ? rangeFromFragment(parsed.fragment)
          : undefined;
        if (!selection && finding.line !== undefined) {
          selection = new vscode.Range(
            finding.line - 1,
            0,
            finding.line - 1,
            0
          );
        } else if (
          !selection &&
          finding.range !== undefined
        ) {
          selection = new vscode.Range(
            finding.range.start - 1,
            0,
            finding.range.end - 1,
            0
          );
        }
      } else {
        uri = vscode.Uri.file(path.join(workspaceRoot, finding.file));
        selection = selectionFromFinding(finding);
      }
    } catch {
      uri = vscode.Uri.file(path.join(workspaceRoot, finding.file));
      selection = selectionFromFinding(finding);
    }
  } else {
    uri = vscode.Uri.file(path.join(workspaceRoot, finding.file));
    selection = selectionFromFinding(finding);
  }

  const options: vscode.TextDocumentShowOptions = selection
    ? { selection, preview: false }
    : { preview: false };
  return vscode.window.showTextDocument(uri, options);
}

function selectionFromFinding(finding: OpenFindingPayload): vscode.Range | undefined {
  if (finding.range !== undefined) {
    return new vscode.Range(
      finding.range.start - 1,
      0,
      finding.range.end - 1,
      0
    );
  }
  if (finding.line !== undefined) {
    const line = finding.line - 1;
    if (line < 0) return undefined;
    return new vscode.Range(line, 0, line, 0);
  }
  return undefined;
}
