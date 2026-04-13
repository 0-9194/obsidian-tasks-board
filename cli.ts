#!/usr/bin/env node
/**
 * cli.ts — obsidian-tasks-board unified CLI
 *
 * Subcommands:
 *   board   Interactive kanban TUI for an Obsidian vault (default)
 *   init    Scaffold a new vault with the otb board layout
 *
 * Usage:
 *   node --experimental-strip-types cli.ts [board] [--vault <path>] [--project <name>]
 *   node --experimental-strip-types cli.ts init [--name <name>] [--dir <path>] [--author <handle>]
 *
 * Requires: Node.js >= 22
 */

import { createInterface } from "node:readline";
import { readBoardData }   from "./src/board-reader.ts";
import { resolveVaultPath, VaultNotFoundError } from "./src/vault-resolver.ts";
import { changeTaskStatus, addTaskComment, FingerprintMismatch } from "./src/writer.ts";
import { initVault }       from "./src/init.ts";
import type { Task, TaskStatus } from "./src/parser.ts";
import type { BoardData }        from "./src/board-reader.ts";

// ── ANSI helpers ──────────────────────────────────────────────────────────────

const A = {
  reset:      "\x1b[0m",
  bold:       "\x1b[1m",
  dim:        "\x1b[2m",
  clear:      "\x1b[2J\x1b[H",
  hideCursor: "\x1b[?25l",
  showCursor: "\x1b[?25h",
  cyan:    "\x1b[36m",
  yellow:  "\x1b[33m",
  green:   "\x1b[32m",
  red:     "\x1b[31m",
  gray:    "\x1b[90m",
};

function c(color: string, text: string): string { return color + text + A.reset; }
function bold(text: string): string { return A.bold + text + A.reset; }
function dim(text: string): string  { return A.dim + A.gray + text + A.reset; }
function termWidth(): number { return process.stdout.columns ?? 80; }
function hr(char = "─", width = termWidth()): string { return c(A.gray, char.repeat(Math.max(0, width))); }
function truncate(s: string, max: number): string {
  if (s.length <= max) return s;
  return s.slice(0, max - 1) + "…";
}

// ── Column definitions ────────────────────────────────────────────────────────

interface Column { id: TaskStatus; label: string; icon: string; }
const COLUMNS: Column[] = [
  { id: "todo",        label: "A Fazer",      icon: "○" },
  { id: "in_progress", label: "Em Progresso", icon: "◐" },
  { id: "done",        label: "Concluído",    icon: "●" },
  { id: "cancelled",   label: "Cancelado",    icon: "✕" },
];
const MOVE_KEYS: Record<string, TaskStatus> = {
  i: "in_progress", t: "todo", d: "done", x: "cancelled",
};
const MAX_TASKS = 12;

// ── Board state ───────────────────────────────────────────────────────────────

let data: BoardData;
let vaultPath: string;
let boardTitle: string;
let col: TaskStatus = "todo";
let idx = 0;
let filterText = "";
let filterProject: string | undefined;
let filterInputOpen = false;
let statusMsg: { text: string; kind: "ok" | "error" } | undefined;

function filteredTasks(status: TaskStatus): Task[] {
  let tasks = data.byStatus[status];
  if (filterProject) tasks = tasks.filter((t) => t.sourceFile.includes(filterProject!));
  const needle = filterText.trim().toLowerCase();
  if (needle) tasks = tasks.filter((t) =>
    t.text.toLowerCase().includes(needle) ||
    (t.type?.toLowerCase().includes(needle) ?? false));
  return tasks;
}
function currentTasks(): Task[] { return filteredTasks(col); }
function selectedTask(): Task | undefined { return currentTasks()[idx]; }
function hasFilter(): boolean { return !!filterProject || filterText.trim() !== ""; }

// ── Render ────────────────────────────────────────────────────────────────────

function render(): void {
  const w = termWidth();
  const lines: string[] = [];

  lines.push(hr());
  const totalLabel = hasFilter()
    ? c(A.yellow, ` ${data.allTasks.filter((t) => filteredTasks(t.status).includes(t)).length}/${data.allTasks.length} `)
    : dim(` ${data.allTasks.length} tarefas `);
  lines.push(bold(c(A.cyan, ` 🗂  ${boardTitle} `)) + totalLabel);

  if (hasFilter() || filterInputOpen) {
    lines.push(c(A.gray, "┄".repeat(Math.max(0, w))));
    const parts: string[] = [];
    if (filterProject) parts.push(c(A.yellow, `  projeto: ${filterProject}`));
    if (filterInputOpen) {
      parts.push(c(A.yellow, `  busca: ${filterText}`) + c(A.cyan, "▌"));
    } else if (filterText) {
      parts.push(c(A.yellow, `  busca: "${filterText}"`));
    }
    lines.push(dim(" 🔍 ") + parts.join(dim("  ·  ")));
  }
  lines.push(hr());

  const tabs = COLUMNS.map((column) => {
    const count = filteredTasks(column.id).length;
    const total = data.byStatus[column.id].length;
    const countLabel = hasFilter() ? `${count}/${total}` : String(count);
    const label = `${column.icon} ${column.label} (${countLabel})`;
    return column.id === col
      ? bold(c(A.cyan, ` ▶ ${label} `))
      : dim(`   ${label}  `);
  }).join(c(A.gray, "│"));
  lines.push(tabs);
  lines.push(hr());

  const tasks = currentTasks();
  if (tasks.length === 0) {
    lines.push(dim(hasFilter()
      ? "  Nenhuma tarefa corresponde ao filtro."
      : "  Nenhuma tarefa nesta coluna."));
  } else {
    const start = Math.max(0, idx - MAX_TASKS + 1);
    const slice = tasks.slice(start, start + MAX_TASKS);
    slice.forEach((task, i) => {
      const absIdx = start + i;
      const isSel = absIdx === idx;
      const cursor = isSel ? c(A.cyan, "▶") : " ";
      const icon = statusIconStr(task.status, isSel);
      const tag = task.type ? dim(` [${task.type}]`) : "";
      const cmtBadge = task.comments.length > 0 ? dim(` 💬${task.comments.length}`) : "";
      const text = isSel ? c(A.cyan, task.text) : task.text;
      lines.push(truncate(` ${cursor} ${icon} ${text}${tag}${cmtBadge}`, w));
    });
    if (tasks.length > MAX_TASKS) lines.push(dim(`    … ${tasks.length - MAX_TASKS} mais`));
  }

  const sel = selectedTask();
  if (sel) {
    lines.push(c(A.gray, "╌".repeat(Math.max(0, w))));
    lines.push(dim("  origem     : ") + sel.sourceFile + ":" + sel.lineNumber);
    if (sel.type)    lines.push(dim("  tipo       : ") + sel.type);
    if (sel.refs)    lines.push(dim("  refs       : ") + c(A.cyan, sel.refs));
    if (sel.comments.length > 0) {
      lines.push(dim(`  comentários (${sel.comments.length}):`));
      const comments = sel.comments.slice(-5);
      if (sel.comments.length > 5)
        lines.push(dim(`    … ${sel.comments.length - 5} anterior(es) omitido(s)`));
      for (const cmt of comments) {
        const [ts, ...rest] = cmt.text.split(" — ");
        const body = rest.join(" — ");
        lines.push(dim("    · ") + c(A.gray, ts ?? cmt.text) + (body ? dim(" — ") + body : ""));
      }
    }
  }

  if (statusMsg) {
    lines.push(c(A.gray, "╌".repeat(Math.max(0, w))));
    const color = statusMsg.kind === "ok" ? A.green : A.red;
    lines.push(c(color, `  ${statusMsg.kind === "ok" ? "✓" : "✗"} ${statusMsg.text}`));
    statusMsg = undefined;
  }

  lines.push(hr());
  if (filterInputOpen) {
    lines.push(dim("  Digite para filtrar") + dim("  Enter: confirmar") + dim("  Esc: cancelar"));
  } else {
    lines.push(
      dim("  Tab/←→: col") + dim("  ↑↓: item") + dim("  i/t/d/x: mover") +
      dim("  c: comentar") + dim("  /: busca") + dim("  p: projeto") +
      dim("  F: limpar") + dim("  r: reload") + dim("  q: sair"),
    );
  }
  lines.push(hr());

  process.stdout.write(A.clear + lines.join("\n") + "\n");
}

function statusIconStr(status: TaskStatus, selected: boolean): string {
  switch (status) {
    case "todo":        return selected ? c(A.cyan, "○") : dim("○");
    case "in_progress": return c(A.yellow, "◐");
    case "done":        return c(A.green, "●");
    case "cancelled":   return c(A.red, "✕");
  }
}

// ── Prompts ───────────────────────────────────────────────────────────────────

async function prompt(question: string): Promise<string> {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  if (process.stdin.isTTY) process.stdin.setRawMode(false);
  return new Promise((resolve) => {
    rl.question(question, (answer) => { rl.close(); if (process.stdin.isTTY) process.stdin.setRawMode(true); resolve(answer); });
  });
}

async function selectProject(): Promise<string | undefined> {
  const names = data.projects.map((p) => p.name);
  if (names.length === 0) return undefined;
  process.stdout.write(A.clear);
  process.stdout.write(bold(c(A.cyan, " Filtrar por projeto\n\n")));
  process.stdout.write("  0) — Todos os projetos —\n");
  names.forEach((n, i) => process.stdout.write(`  ${i + 1}) ${n}\n`));
  process.stdout.write("\n");
  const answer = await prompt("  Escolha (número): ");
  const n = parseInt(answer.trim(), 10);
  if (n === 0) return "";
  if (n >= 1 && n <= names.length) return names[n - 1];
  return undefined;
}

async function confirmPrompt(question: string): Promise<boolean> {
  const answer = await prompt(`${question} [s/N] `);
  return answer.trim().toLowerCase() === "s";
}

// ── Actions ───────────────────────────────────────────────────────────────────

async function handleMove(newStatus: TaskStatus): Promise<void> {
  const task = selectedTask();
  if (!task || task.status === newStatus) return;
  const label = COLUMNS.find((col) => col.id === newStatus)!.label;
  process.stdout.write(A.clear);
  const ok = await confirmPrompt(`Mover "${task.text}" → ${label}?`);
  if (!ok) { render(); return; }
  try {
    await changeTaskStatus(vaultPath, task, newStatus);
    await reloadData(`Status atualizado → ${label}`);
  } catch (err) { reportError(err); }
  render();
}

async function handleComment(): Promise<void> {
  const task = selectedTask();
  if (!task) return;
  process.stdout.write(A.clear);
  const text = await prompt(`Comentário para "${task.text}": `);
  if (!text.trim()) { render(); return; }
  try {
    await addTaskComment(vaultPath, task, text.trim());
    await reloadData("Comentário adicionado");
  } catch (err) { reportError(err); }
  render();
}

async function handleProjectFilter(): Promise<void> {
  const choice = await selectProject();
  if (choice === undefined) { render(); return; }
  if (choice === "") {
    filterProject = undefined;
    statusMsg = { text: "Mostrando todos os projetos", kind: "ok" };
  } else {
    filterProject = choice;
    statusMsg = { text: `Projeto: ${choice}`, kind: "ok" };
  }
  idx = 0;
  render();
}

async function reloadData(msg: string): Promise<void> {
  data = await readBoardData(vaultPath);
  const tasks = filteredTasks(col);
  if (idx >= tasks.length) idx = Math.max(0, tasks.length - 1);
  statusMsg = { text: msg, kind: "ok" };
}

function reportError(err: unknown): void {
  const msg = err instanceof FingerprintMismatch
    ? "Arquivo mudou — recarregue (r) e tente novamente."
    : err instanceof Error ? err.message : String(err);
  statusMsg = { text: `Erro: ${msg}`, kind: "error" };
}

// ── Input loop ────────────────────────────────────────────────────────────────

async function handleKey(key: Buffer): Promise<boolean> {
  const s = key.toString();

  if (filterInputOpen) {
    if (s === "\x1b" || s === "\x03") { filterInputOpen = false; filterText = ""; idx = 0; render(); return false; }
    if (s === "\r" || s === "\n")      { filterInputOpen = false; idx = 0; render(); return false; }
    if (s === "\x7f" || s === "\x08")  { filterText = filterText.slice(0, -1); idx = 0; render(); return false; }
    if (s.length === 1 && s >= " ")    { filterText += s; idx = 0; render(); return false; }
    return false;
  }

  if (s === "q" || s === "\x03") return true;

  if (s === "\x1b") {
    if (hasFilter()) { filterProject = undefined; filterText = ""; idx = 0; render(); }
    else return true;
    return false;
  }

  if (s === "\x1b[C" || s === "\t") {
    const i = COLUMNS.findIndex((c) => c.id === col);
    col = COLUMNS[(i + 1) % COLUMNS.length]!.id; idx = 0; render(); return false;
  }
  if (s === "\x1b[D" || s === "\x1b[Z") {
    const i = COLUMNS.findIndex((c) => c.id === col);
    col = COLUMNS[(i - 1 + COLUMNS.length) % COLUMNS.length]!.id; idx = 0; render(); return false;
  }
  if (s === "\x1b[A") { if (idx > 0) { idx--; render(); } return false; }
  if (s === "\x1b[B") { const max = currentTasks().length - 1; if (idx < max) { idx++; render(); } return false; }

  if (s === "r") { try { await reloadData("Board recarregado"); } catch (err) { reportError(err); } render(); return false; }
  if (s === "c") { await handleComment(); return false; }
  if (s === "/" || s === "f") { filterInputOpen = true; render(); return false; }
  if (s === "p") { await handleProjectFilter(); return false; }
  if (s === "F") { filterProject = undefined; filterText = ""; statusMsg = { text: "Filtros removidos", kind: "ok" }; idx = 0; render(); return false; }

  const target = MOVE_KEYS[s.toLowerCase()];
  if (target !== undefined) { await handleMove(target); return false; }
  return false;
}

// ── CLI arg parser ────────────────────────────────────────────────────────────

interface ParsedArgs {
  subcommand: "board" | "init";
  // board
  vault?:   string;
  project?: string;
  // init
  name?:    string;
  dir?:     string;
  author?:  string;
  force?:   boolean;
}

function parseArgs(): ParsedArgs {
  const raw = process.argv.slice(2);
  const result: ParsedArgs = { subcommand: "board" };

  let i = 0;
  // first positional arg can be subcommand
  if (raw[0] && !raw[0].startsWith("-")) {
    const sub = raw[0].toLowerCase();
    if (sub === "init" || sub === "board") { result.subcommand = sub; i = 1; }
  }

  for (; i < raw.length; i++) {
    const arg = raw[i]!;
    if ((arg === "--vault"  || arg === "-v") && raw[i + 1]) { result.vault   = raw[++i]; continue; }
    if ((arg === "--project"|| arg === "-p") && raw[i + 1]) { result.project = raw[++i]; continue; }
    if ((arg === "--name"   || arg === "-n") && raw[i + 1]) { result.name    = raw[++i]; continue; }
    if ((arg === "--dir"    || arg === "-d") && raw[i + 1]) { result.dir     = raw[++i]; continue; }
    if ((arg === "--author" || arg === "-a") && raw[i + 1]) { result.author  = raw[++i]; continue; }
    if (arg === "--force" || arg === "-f")  { result.force = true; continue; }
  }
  return result;
}

// ── Subcommand: board ─────────────────────────────────────────────────────────

async function runBoard(args: ParsedArgs): Promise<void> {
  // Resolve vault
  try {
    vaultPath = await resolveVaultPath(process.cwd(), args.vault);
  } catch (err) {
    const msg = err instanceof VaultNotFoundError ? err.message
      : err instanceof Error ? err.message : String(err);
    console.error(msg);
    process.exit(1);
  }

  // Board title from vault directory name
  const { basename } = await import("node:path");
  boardTitle = basename(vaultPath);

  // Load board
  try {
    data = await readBoardData(vaultPath);
  } catch (err) {
    console.error("Erro ao ler vault:", err instanceof Error ? err.message : err);
    process.exit(1);
  }

  if (data.allTasks.length === 0) {
    console.log("Nenhuma tarefa encontrada no vault.");
    process.exit(0);
  }

  // Apply --project filter
  if (args.project) {
    const match = data.projects.find((p) =>
      p.name.toLowerCase().includes(args.project!.toLowerCase()));
    if (match) {
      filterProject = match.name;
    } else {
      console.error(`Projeto não encontrado: "${args.project}"`);
      process.exit(1);
    }
  }

  col = COLUMNS.find((c) => filteredTasks(c.id).length > 0)?.id ?? "todo";

  if (!process.stdin.isTTY) {
    console.error("board requer um terminal interativo (TTY).");
    process.exit(1);
  }

  process.stdin.setRawMode(true);
  process.stdin.resume();
  process.stdout.write(A.hideCursor);

  const cleanup = () => {
    process.stdout.write(A.showCursor + A.reset + "\n");
    process.stdin.setRawMode(false);
    process.exit(0);
  };
  process.on("SIGINT", cleanup);
  process.on("SIGTERM", cleanup);

  render();

  for await (const key of process.stdin) {
    const quit = await handleKey(key as Buffer);
    if (quit) break;
  }

  cleanup();
}

// ── Subcommand: init ──────────────────────────────────────────────────────────

async function runInit(args: ParsedArgs): Promise<void> {
  try {
    await initVault(process.cwd(), {
      name:   args.name,
      dir:    args.dir,
      author: args.author,
      force:  args.force,
    });
  } catch (err) {
    console.error("Erro ao criar vault:", err instanceof Error ? err.message : err);
    process.exit(1);
  }
}

// ── Main ──────────────────────────────────────────────────────────────────────

async function main(): Promise<void> {
  const args = parseArgs();
  if (args.subcommand === "init") {
    await runInit(args);
  } else {
    await runBoard(args);
  }
}

main().catch((err) => {
  console.error("Erro fatal:", err);
  process.exit(1);
});
