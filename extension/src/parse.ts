/**
 * Parses CLI stdout (JSON or NDJSON) into findings per docs/cli-extension-contract.md.
 */

import type { Finding, FindingsResponse, LineRange, Severity, Category } from "./contract";
import { SEVERITIES, CATEGORIES } from "./contract";

/** Stream event: one line of NDJSON when CLI is run with --stream. */
export type StreamEvent =
  | { type: "progress"; msg: string }
  | { type: "finding"; data: Finding }
  | { type: "done" };

function isSeverity(s: string): s is Severity {
  return (SEVERITIES as readonly string[]).includes(s);
}

function isCategory(s: string): s is Category {
  return (CATEGORIES as readonly string[]).includes(s);
}

function validateLineRange(r: unknown): r is LineRange {
  if (r === null || typeof r !== "object") return false;
  const o = r as Record<string, unknown>;
  return (
    typeof o.start === "number" &&
    typeof o.end === "number" &&
    Number.isInteger(o.start) &&
    Number.isInteger(o.end)
  );
}

function validateFinding(raw: unknown): raw is Finding {
  if (raw === null || typeof raw !== "object") return false;
  const o = raw as Record<string, unknown>;
  if (typeof o.file !== "string" || o.file === "") return false;
  if (typeof o.message !== "string") return false;
  if (!isSeverity(String(o.severity))) return false;
  if (!isCategory(String(o.category))) return false;
  if (typeof o.confidence !== "number" || o.confidence < 0 || o.confidence > 1) return false;
  if (o.id !== undefined && typeof o.id !== "string") return false;
  if (o.line !== undefined && (typeof o.line !== "number" || !Number.isInteger(o.line))) return false;
  if (o.range !== undefined && !validateLineRange(o.range)) return false;
  if (o.suggestion !== undefined && typeof o.suggestion !== "string") return false;
  if (o.cursor_uri !== undefined && typeof o.cursor_uri !== "string") return false;
  return true;
}

/**
 * Parses a single JSON object from stdout (current CLI contract: one line).
 * @param stdout - Raw stdout string (single line or trimmed)
 * @returns Parsed findings response
 * @throws Error if JSON is invalid or shape is wrong
 */
export function parseFindingsJSON(stdout: string): FindingsResponse {
  const trimmed = stdout.trim();
  if (trimmed === "") {
    throw new Error("Empty stdout");
  }
  let data: unknown;
  try {
    data = JSON.parse(trimmed);
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e);
    throw new Error(`Invalid JSON: ${message}`);
  }
  if (data === null || typeof data !== "object" || Array.isArray(data)) {
    throw new Error("Expected JSON object");
  }
  const obj = data as Record<string, unknown>;
  if (!Array.isArray(obj.findings)) {
    throw new Error("Missing or invalid 'findings' array");
  }
  const findings: Finding[] = [];
  for (let i = 0; i < obj.findings.length; i++) {
    const item = obj.findings[i];
    if (!validateFinding(item)) {
      throw new Error(`Invalid finding at index ${i}`);
    }
    findings.push(item);
  }
  return { findings };
}

/**
 * Parses NDJSON from stdout (one JSON object per line). Each line may be
 * {"findings": [...]}; findings from all lines are merged.
 * @param stdout - Raw stdout (multiple lines)
 * @returns All findings from every line
 * @throws Error if any line is invalid
 */
export function parseFindingsNDJSON(stdout: string): Finding[] {
  const lines = stdout.split("\n").map((l) => l.trim()).filter((l) => l !== "");
  const allFindings: Finding[] = [];
  for (let i = 0; i < lines.length; i++) {
    let data: unknown;
    try {
      data = JSON.parse(lines[i]);
    } catch (e) {
      const message = e instanceof Error ? e.message : String(e);
      throw new Error(`Line ${i + 1}: Invalid JSON: ${message}`);
    }
    if (data === null || typeof data !== "object") {
      throw new Error(`Line ${i + 1}: Expected JSON object`);
    }
    const obj = data as Record<string, unknown>;
    if (!Array.isArray(obj.findings)) {
      throw new Error(`Line ${i + 1}: Missing or invalid 'findings' array`);
    }
    for (let j = 0; j < obj.findings.length; j++) {
      const item = obj.findings[j];
      if (!validateFinding(item)) {
        throw new Error(`Line ${i + 1}, finding ${j}: Invalid finding`);
      }
      allFindings.push(item);
    }
  }
  return allFindings;
}

/**
 * Parses a single NDJSON stream event line (--stream output).
 * @param line - One line of stdout (trimmed)
 * @returns StreamEvent (progress, finding, or done)
 * @throws Error if JSON is invalid, type is unknown, or finding data is invalid
 */
export function parseStreamEvent(line: string): StreamEvent {
  const trimmed = line.trim();
  if (trimmed === "") {
    throw new Error("Empty line");
  }
  let data: unknown;
  try {
    data = JSON.parse(trimmed);
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e);
    throw new Error(`Invalid JSON: ${message}`);
  }
  if (data === null || typeof data !== "object" || Array.isArray(data)) {
    throw new Error("Expected JSON object");
  }
  const obj = data as Record<string, unknown>;
  const type = obj.type;
  if (type === "progress") {
    if (typeof obj.msg !== "string") {
      throw new Error("progress event missing or invalid msg");
    }
    return { type: "progress", msg: obj.msg };
  }
  if (type === "finding") {
    if (!validateFinding(obj.data)) {
      throw new Error("Invalid finding");
    }
    return { type: "finding", data: obj.data };
  }
  if (type === "done") {
    return { type: "done" };
  }
  throw new Error(`Unknown stream event type: ${String(type)}`);
}
