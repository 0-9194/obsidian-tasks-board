/**
 * board-reader.ts — Reads markdown files from a vault and aggregates tasks.
 *
 * Configurable via BoardReaderConfig:
 *   - scanDirs   — directories to scan (relative to vault root)
 *   - excluded   — filenames to skip (e.g. global board view files)
 *
 * Defaults match the otb default layout but any similar Obsidian vault works.
 */

import { readFile, readdir, stat } from "node:fs/promises";
import { join, basename } from "node:path";
import { parseProjectFile, type Task, type TaskStatus } from "./parser.ts";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface BoardReaderConfig {
  /** Directories to scan, relative to vault root. Default: otb default layout. */
  scanDirs?: string[];
  /** Filenames (basename) to exclude from scanning. */
  excluded?: string[];
}

export interface ProjectSummary {
  name: string;
  file: string;
  relativePath: string;
  tasks: Task[];
}

export interface BoardData {
  projects: ProjectSummary[];
  allTasks: Task[];
  byStatus: Record<TaskStatus, Task[]>;
}

// ── Defaults ──────────────────────────────────────────────────────────────────

const DEFAULT_SCAN_DIRS = ["20 - Projects", "docs"];
const DEFAULT_EXCLUDED  = new Set(["Board Global.md"]);

// ── Primitives ────────────────────────────────────────────────────────────────

export async function discoverFiles(dir: string, excluded: Set<string>): Promise<string[]> {
  let entries: string[];
  try { entries = await readdir(dir); } catch { return []; }

  const paths: string[] = [];
  for (const entry of entries.sort()) {
    if (!entry.endsWith(".md") || excluded.has(entry)) continue;
    const fullPath = join(dir, entry);
    try {
      const s = await stat(fullPath);
      if (s.isFile()) paths.push(fullPath);
    } catch { /* skip unreadable */ }
  }
  return paths;
}

export async function loadFile(
  filePath: string,
  vaultRoot: string,
): Promise<ProjectSummary | null> {
  let content: string;
  try { content = await readFile(filePath, "utf-8"); } catch { return null; }

  const relativePath = filePath.startsWith(vaultRoot)
    ? filePath.slice(vaultRoot.length).replace(/^\//, "")
    : basename(filePath);

  const tasks = parseProjectFile(content, relativePath);
  return { name: basename(filePath, ".md"), file: basename(filePath), relativePath, tasks };
}

// ── Public API ────────────────────────────────────────────────────────────────

export async function readBoardData(
  vaultPath: string,
  config: BoardReaderConfig = {},
): Promise<BoardData> {
  const scanDirs = config.scanDirs ?? DEFAULT_SCAN_DIRS;
  const excluded = config.excluded ? new Set(config.excluded) : DEFAULT_EXCLUDED;

  const projects: ProjectSummary[] = [];

  for (const dirName of scanDirs) {
    const dir = join(vaultPath, dirName);
    const files = await discoverFiles(dir, excluded);
    for (const filePath of files) {
      const summary = await loadFile(filePath, vaultPath);
      if (summary) projects.push(summary);
    }
  }

  const allTasks = projects.flatMap((p) => p.tasks);
  return {
    projects,
    allTasks,
    byStatus: {
      todo:        allTasks.filter((t) => t.status === "todo"),
      in_progress: allTasks.filter((t) => t.status === "in_progress"),
      done:        allTasks.filter((t) => t.status === "done"),
      cancelled:   allTasks.filter((t) => t.status === "cancelled"),
      backlog:     allTasks.filter((t) => t.status === "backlog"),
    },
  };
}
