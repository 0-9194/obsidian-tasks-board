/**
 * parser.ts — Markdown task parser for Obsidian Tasks plugin syntax.
 *
 * Parses task lines from vault markdown files.
 * Supports inline fields: [type:: ...], [refs:: ...]
 * Attaches indented `comment::` subtopics to their parent task.
 */

import { sanitizeForDisplay } from "./sanitize.ts";

// ── Types ─────────────────────────────────────────────────────────────────────

export type TaskStatus = "todo" | "in_progress" | "done" | "cancelled" | "backlog";

export interface TaskComment {
  text: string;
  lineNumber: number;
}

export interface Task {
  /** Display text (inline fields stripped, sanitized) */
  text: string;
  status: TaskStatus;
  type?: string;
  refs?: string;
  comments: TaskComment[];
  sourceFile: string;
  lineNumber: number;
  /**
   * Stable identity key for safe mutations.
   * Format: "<sourceFile>:L<lineNumber>:<normalizedText>"
   */
  fingerprint: string;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function toStatus(checkChar: string): TaskStatus {
  switch (checkChar) {
    case " ": return "todo";
    case "/": return "in_progress";
    case "x":
    case "X": return "done";
    case "-": return "cancelled";
    case "b": return "backlog";
    default:  return "todo";
  }
}

const RE_FIELD_TYPE = /\[type::\s*([^\]]+)\]/;
const RE_FIELD_REFS = /\[refs::\s*([^\]]+)\]/;

function extractField(text: string, field: "type" | "refs"): string | undefined {
  const re = field === "type" ? RE_FIELD_TYPE : RE_FIELD_REFS;
  const m = text.match(re);
  return m ? sanitizeForDisplay(m[1]!.trim(), 100) : undefined;
}

function stripFields(text: string): string {
  return text.replace(/\[[^\]]+::\s*[^\]]+\]/g, "").replace(/\s+/g, " ").trim();
}

function buildFingerprint(sourceFile: string, lineNumber: number, normalizedText: string): string {
  return `${sourceFile}:L${lineNumber}:${normalizedText}`;
}

function parseCommentLine(line: string, lineNumber: number): TaskComment | null {
  const m = line.match(/^(\s{2,})[-*]\s+comment::\s+(.+)$/);
  if (!m) return null;
  const text = sanitizeForDisplay(m[2]!.trim(), 200);
  if (!text) return null;
  return { text, lineNumber };
}

// ── Public API ────────────────────────────────────────────────────────────────

export function parseTaskLine(
  line: string,
  sourceFile: string,
  lineNumber: number,
): Task | null {
  const m = line.match(/^\s*[-*]\s+\[([^\]]*)\]\s+(.+)$/);
  if (!m) return null;
  const checkChar = m[1]!;
  if (checkChar.length > 1) return null;

  const rawSanitized = sanitizeForDisplay(m[2]!, 300);
  const displayText = sanitizeForDisplay(stripFields(rawSanitized), 200);

  return {
    text: displayText,
    status: toStatus(checkChar),
    type: extractField(rawSanitized, "type"),
    refs: extractField(rawSanitized, "refs"),
    comments: [],
    sourceFile,
    lineNumber,
    fingerprint: buildFingerprint(sourceFile, lineNumber, displayText),
  };
}

export function parseProjectFile(content: string, sourceFile: string): Task[] {
  const tasks: Task[] = [];
  const lines = content.split("\n");
  let lastTask: Task | null = null;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]!;
    const lineNumber = i + 1;

    const comment = lastTask ? parseCommentLine(line, lineNumber) : null;
    if (comment) { lastTask!.comments.push(comment); continue; }

    const task = parseTaskLine(line, sourceFile, lineNumber);
    if (task) { tasks.push(task); lastTask = task; continue; }

    if (line.trim() !== "") lastTask = null;
  }

  return tasks;
}
