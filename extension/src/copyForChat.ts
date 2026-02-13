import * as path from "path";
import { pathToFileURL } from "node:url";

import type { Finding } from "./contract";
import type { Category, Severity } from "./contract";

/**
 * Maps CLI severity to markdown admonition type (PRD §3e).
 * error/warning → WARNING; info/nitpick → NOTE.
 */
function severityToAdmonition(severity: Severity): string {
  switch (severity) {
    case "error":
      return "WARNING";
    case "warning":
      return "WARNING";
    case "info":
    case "nitpick":
      return "NOTE";
    default:
      return "NOTE";
  }
}

/**
 * Capitalizes category for blockquote title (e.g. "maintainability" → "Maintainability").
 */
function categoryToTitle(category: Category): string {
  if (category.length === 0) return "Finding";
  return category.charAt(0).toUpperCase() + category.slice(1).replace(/_/g, " ");
}

/**
 * Line number (or range start) for the file link fragment. Fallback 1 if missing.
 */
function lineForFragment(finding: Finding): number {
  if (finding.line !== undefined) return finding.line;
  if (finding.range?.start !== undefined) return finding.range.start;
  return 1;
}

/**
 * Line part for link text: "10" or "5-7".
 */
function linePartForLabel(finding: Finding): string {
  if (finding.range !== undefined) {
    return `${finding.range.start}-${finding.range.end}`;
  }
  if (finding.line !== undefined) return String(finding.line);
  return "1";
}

/**
 * Builds the PRD §3e "Copy for Chat" markdown block for a finding.
 * Format: [file:line](file:///abs/path#L10), then blockquote with [!SEVERITY] title and message.
 *
 * @param finding - The finding (file, line/range, severity, category, message).
 * @param workspaceRoot - Absolute path to the workspace root (used to build file URI).
 * @returns Markdown string suitable for pasting into Cursor Chat.
 */
export function buildCopyForChatBlock(
  finding: Finding,
  workspaceRoot: string
): string {
  const lineNum = lineForFragment(finding);
  const linePart = linePartForLabel(finding);
  const absPath = path.join(workspaceRoot, finding.file);
  const fileUrl = pathToFileURL(absPath).toString() + `#L${lineNum}`;
  const linkText = `${finding.file}:${linePart}`;
  const linkLine = `[${linkText}](${fileUrl})`;

  const admonition = severityToAdmonition(finding.severity);
  const title = categoryToTitle(finding.category);
  const headerLine = `> [!${admonition}] ${title}`;

  const messageLines = finding.message
    .split(/\r?\n/)
    .map((line) => `> ${line}`)
    .join("\n");

  return `${linkLine}\n${headerLine}\n${messageLines}`;
}
