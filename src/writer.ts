/**
 * writer.ts — Safe, atomic mutations to vault Markdown task lines.
 *
 * changeTaskStatus()   swap the checkbox char on a specific line
 * addTaskComment()     append a comment:: subtopic below a task
 *
 * Safety model:
 *   1. Read the file fresh before writing.
 *   2. Verify the target line still matches the task's fingerprint.
 *   3. Apply the minimal change.
 *   4. Write atomically (temp file + rename).
 */

import { readFile, writeFile, rename } from "node:fs/promises";
import { join, dirname } from "node:path";
import { randomBytes } from "node:crypto";
import type { Task, TaskStatus } from "./parser.ts";

const STATUS_TO_CHAR: Record<TaskStatus, string> = {
  todo: " ", in_progress: "/", done: "x", cancelled: "-",
};

function normalizeLine(raw: string): string {
  return raw.replace(/\[[^\]]+::\s*[^\]]+\]/g, "").replace(/\s+/g, " ").trim();
}

async function readLines(filePath: string): Promise<{ lines: string[]; eol: string }> {
  const raw = await readFile(filePath, "utf-8");
  const eol = raw.includes("\r\n") ? "\r\n" : "\n";
  return { lines: raw.split(eol === "\r\n" ? /\r?\n/ : "\n"), eol };
}

async function writeLines(filePath: string, lines: string[], eol: string): Promise<void> {
  const dir = dirname(filePath);
  const tmp = join(dir, `.tmp-writer-${randomBytes(6).toString("hex")}.md`);
  await writeFile(tmp, lines.join(eol), "utf-8");
  await rename(tmp, filePath);
}

// ── Verification ──────────────────────────────────────────────────────────────

export class FingerprintMismatch extends Error {
  readonly task: Task;
  readonly actualLine: string;
  constructor(task: Task, actualLine: string) {
    super(
      `Fingerprint mismatch for task at ${task.sourceFile}:${task.lineNumber}.\n` +
      `  Expected text : "${task.text}"\n` +
      `  Actual line   : "${actualLine.trim()}"`,
    );
    this.name = "FingerprintMismatch";
    this.task = task;
    this.actualLine = actualLine;
  }
}

function verifyLine(lines: string[], task: Task): void {
  const zero = task.lineNumber - 1;
  const actual = lines[zero];
  if (actual === undefined) throw new FingerprintMismatch(task, "<line does not exist>");

  const m = actual.match(/^\s*[-*]\s+\[[^\]]*\]\s+(.+)$/);
  if (!m) throw new FingerprintMismatch(task, actual);

  const actualNorm = normalizeLine(m[1]!);
  const expectedNorm = normalizeLine(task.text);

  if (actualNorm !== expectedNorm) {
    const prefix = expectedNorm.substring(0, Math.min(20, expectedNorm.length));
    if (prefix.length < 5 || !actualNorm.startsWith(prefix))
      throw new FingerprintMismatch(task, actual);
  }
}

// ── Public API ────────────────────────────────────────────────────────────────

export async function changeTaskStatus(
  vaultPath: string,
  task: Task,
  newStatus: TaskStatus,
): Promise<void> {
  if (task.status === newStatus) return;
  const filePath = join(vaultPath, task.sourceFile);
  const { lines, eol } = await readLines(filePath);
  verifyLine(lines, task);

  const zero = task.lineNumber - 1;
  const updated = lines[zero]!.replace(/^(\s*[-*]\s+\[)[^\]]*(\])/, `$1${STATUS_TO_CHAR[newStatus]}$2`);
  if (updated === lines[zero]) throw new Error(`Could not locate checkbox on line ${task.lineNumber}`);

  lines[zero] = updated;
  await writeLines(filePath, lines, eol);
}

export async function addTaskComment(
  vaultPath: string,
  task: Task,
  text: string,
  author = "user",
): Promise<void> {
  const filePath = join(vaultPath, task.sourceFile);
  const { lines, eol } = await readLines(filePath);
  verifyLine(lines, task);

  const safe = text.replace(/[\x00-\x1f\x7f]/g, "").trim().slice(0, 200);
  if (!safe) throw new Error("Comment text is empty after sanitization.");

  const now = new Date();
  const pad = (n: number) => String(n).padStart(2, "0");
  const ts = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}`;
  const commentLine = `  - comment:: ${ts} @${author} — ${safe}`;

  let insertAt = task.lineNumber;
  while (insertAt < lines.length && /^\s{2,}[-*]\s+comment::/.test(lines[insertAt]!)) insertAt++;

  lines.splice(insertAt, 0, commentLine);
  await writeLines(filePath, lines, eol);
}
