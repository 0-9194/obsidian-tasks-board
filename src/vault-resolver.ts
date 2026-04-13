/**
 * vault-resolver.ts — Resolves the absolute path to an Obsidian vault.
 *
 * Detection order (first match wins):
 *   1. Explicit --vault argument (passed in directly)
 *   2. <cwd>/.obsidian                     — cwd IS the vault
 *   3. Any single child with .obsidian/    — cwd contains exactly one vault
 *
 * The presence of `.obsidian/` is the canonical marker for an Obsidian vault.
 * This module has no hardcoded vault names.
 */

import { stat, readdir } from "node:fs/promises";
import { join } from "node:path";

async function isDir(p: string): Promise<boolean> {
  try { return (await stat(p)).isDirectory(); } catch { return false; }
}

export class VaultNotFoundError extends Error {
  constructor(cwd: string) {
    super(
      `No Obsidian vault found.\n` +
      `  Searched: "${cwd}" (direct vault or parent with single vault child)\n\n` +
      `  Solutions:\n` +
      `    • Run from inside the vault directory (must contain .obsidian/)\n` +
      `    • Run from a directory that contains exactly one vault subfolder\n` +
      `    • Pass the vault path explicitly: --vault /path/to/vault`,
    );
    this.name = "VaultNotFoundError";
  }
}

/**
 * Returns the absolute path to the vault root, or throws VaultNotFoundError.
 *
 * @param cwd       - Current working directory (or any base path)
 * @param explicit  - Explicit path from --vault flag; returned as-is if provided
 */
export async function resolveVaultPath(cwd: string, explicit?: string): Promise<string> {
  // 0. Explicit --vault flag wins immediately
  if (explicit) {
    if (await isDir(join(explicit, ".obsidian"))) return explicit;
    throw new Error(
      `Path "${explicit}" does not appear to be an Obsidian vault (missing .obsidian/).`,
    );
  }

  // 1. cwd itself is the vault
  if (await isDir(join(cwd, ".obsidian"))) return cwd;

  // 2. Scan cwd children — accept if exactly one vault is found
  let entries: string[];
  try { entries = await readdir(cwd); } catch { entries = []; }

  const vaults: string[] = [];
  for (const entry of entries) {
    const candidate = join(cwd, entry);
    if (await isDir(join(candidate, ".obsidian"))) vaults.push(candidate);
  }

  if (vaults.length === 1) return vaults[0]!;

  throw new VaultNotFoundError(cwd);
}
