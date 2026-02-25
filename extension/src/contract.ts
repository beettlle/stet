/**
 * Types matching the CLI–extension contract (docs/cli-extension-contract.md).
 * Single source of truth for parsing stet stdout.
 */

export const SEVERITIES = [
  "error",
  "warning",
  "info",
  "nitpick",
] as const;
export type Severity = (typeof SEVERITIES)[number];

export const CATEGORIES = [
  "bug",
  "security",
  "correctness",
  "performance",
  "style",
  "maintainability",
  "best_practice",
  "testing",
  "documentation",
  "design",
  "accessibility",
] as const;
export type Category = (typeof CATEGORIES)[number];

export interface LineRange {
  start: number;
  end: number;
}

export interface Finding {
  id?: string;
  file: string;
  line?: number;
  range?: LineRange;
  severity: Severity;
  category: Category;
  confidence: number;
  message: string;
  suggestion?: string;
  cursor_uri?: string;
}

export interface FindingsResponse {
  findings: Finding[];
}
