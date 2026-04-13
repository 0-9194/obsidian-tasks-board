/**
 * init.ts — Scaffold a new Obsidian vault with the otb board layout.
 *
 * Creates the directory structure, configuration files, and starter notes
 * needed for obsidian-tasks-board to work out of the box.
 *
 * Usage (via CLI):
 *   otb init                        # scaffold in current directory
 *   otb init --name "My Project"    # named vault in ./my-project/
 *   otb init --dir /path/to/vault   # explicit output directory
 */

import { mkdir, writeFile, access } from "node:fs/promises";
import { join, resolve } from "node:path";
import { constants } from "node:fs";

// ── Types ─────────────────────────────────────────────────────────────────────

export interface InitOptions {
  /** Vault name used in headings and as directory name (default: "my-vault") */
  name?: string;
  /** Target directory. Defaults to ./<slug(name)> relative to cwd */
  dir?: string;
  /** Author handle used in templates (default: "user") */
  author?: string;
  /** Overwrite existing files (default: false) */
  force?: boolean;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function slug(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

function today(): string {
  return new Date().toISOString().slice(0, 10);
}

async function exists(p: string): Promise<boolean> {
  try { await access(p, constants.F_OK); return true; } catch { return false; }
}

async function mkdirp(p: string): Promise<void> {
  await mkdir(p, { recursive: true });
}

async function writeIfNew(
  filePath: string,
  content: string,
  force: boolean,
  log: (msg: string) => void,
): Promise<void> {
  if (!force && await exists(filePath)) {
    log(`  skip  ${filePath} (already exists)`);
    return;
  }
  await writeFile(filePath, content, "utf-8");
  log(`  create ${filePath}`);
}

// ── File templates ────────────────────────────────────────────────────────────

function obsidianAppJson(): string {
  return JSON.stringify({
    "legacyEditorEnabled": false,
    "livePreview": true,
  }, null, 2) + "\n";
}

function obsidianPluginsJson(): string {
  // Minimal plugins entry — user must install Tasks + Dataview manually in Obsidian
  return JSON.stringify({}, null, 2) + "\n";
}

function boardGlobalMd(name: string): string {
  return `---
title: Board Global
aliases:
  - Board
  - Kanban Global
tags:
  - board
status: active
created: ${today()}
updated: ${today()}
category: board
---

# 🗂️ Board Global — ${name}

> View dinâmica de todas as tarefas dos projetos e documentação.
> **Não edite tarefas aqui** — vá até a nota de origem.

---

> [!todo]+ 📋 A Fazer
>
> \`\`\`tasks
> not done
> filter by function task.status.symbol === ' '
> (path includes 20 - Projects) OR (path includes docs)
> group by filename
> sort by priority
> \`\`\`

> [!warning]+ 🔄 Em Progresso
>
> \`\`\`tasks
> not done
> filter by function task.status.symbol === '/'
> (path includes 20 - Projects) OR (path includes docs)
> group by filename
> sort by priority
> \`\`\`

> [!success]+ ✅ Concluído
>
> \`\`\`tasks
> done
> (path includes 20 - Projects) OR (path includes docs)
> group by filename
> sort by done date reverse
> \`\`\`

> [!failure]+ ✕ Cancelado
>
> \`\`\`tasks
> not done
> filter by function task.status.symbol === '-'
> (path includes 20 - Projects) OR (path includes docs)
> group by filename
> \`\`\`
`;
}

function projectTemplateMd(name: string): string {
  return `---
title: "{{title}}"
tags:
  - projeto
  - status/em-progresso
status: em-progresso
created: {{date}}
updated: {{date}}
prazo:
responsavel:
type: técnico
---

# 🚀 {{title}}

## 🎯 Objetivo

_Qual é o resultado final desejado? O que define o sucesso deste projeto?_

## 🔗 Contexto e referências

_Links para ADRs, recursos ou dependências relevantes._

---

## 📋 Board do Projeto

> [!todo]- 📋 A Fazer
> \`\`\`tasks
> not done
> filter by function task.status.symbol === ' '
> path includes 20 - Projects/{{title}}
> sort by priority
> \`\`\`

> [!warning]- 🔄 Em Progresso
> \`\`\`tasks
> not done
> filter by function task.status.symbol === '/'
> path includes 20 - Projects/{{title}}
> sort by priority
> \`\`\`

> [!success]- ✅ Concluído
> \`\`\`tasks
> done
> path includes 20 - Projects/{{title}}
> sort by done date reverse
> \`\`\`

---

## 📝 Tarefas

_Sintaxe: \`- [ ] Descrição [type:: técnico|estratégico] [refs:: PR#N, commit:hash]\`_

### A Fazer

- [ ] 

---

## 🤔 Decisões e Anotações

_Registre decisões pontuais._

---

## 🔮 Próximos Passos

_O que vem depois da conclusão deste projeto?_
`;
}

function starterProjectMd(name: string, projectTitle: string): string {
  return `---
title: "${projectTitle}"
tags:
  - projeto
  - status/em-progresso
status: em-progresso
created: ${today()}
updated: ${today()}
type: técnico
---

# 🚀 ${projectTitle}

## 🎯 Objetivo

_Descreva o objetivo deste projeto aqui._

---

## 📋 Board do Projeto

> [!todo]- 📋 A Fazer
> \`\`\`tasks
> not done
> filter by function task.status.symbol === ' '
> path includes 20 - Projects/${projectTitle}
> sort by priority
> \`\`\`

> [!warning]- 🔄 Em Progresso
> \`\`\`tasks
> not done
> filter by function task.status.symbol === '/'
> path includes 20 - Projects/${projectTitle}
> sort by priority
> \`\`\`

> [!success]- ✅ Concluído
> \`\`\`tasks
> done
> path includes 20 - Projects/${projectTitle}
> sort by done date reverse
> \`\`\`

---

## 📝 Tarefas

- [ ] Primeira tarefa [type:: técnico]
- [ ] Segunda tarefa [type:: estratégico]
- [/] Tarefa em progresso [type:: técnico]
`;
}

function readmeMd(name: string): string {
  return `# ${name}

Vault Obsidian gerenciado com [obsidian-tasks-board](https://github.com/pot-labs/otb).

## Estrutura

\`\`\`
00 - Inbox/          rascunhos e capturas não processadas
10 - Fleeting/       notas diárias e efêmeras
20 - Projects/       projetos ativos com board de tarefas embutido  ← board lê daqui
30 - Areas/          responsabilidades contínuas
40 - Resources/      conhecimento atômico e guias de referência
50 - Archives/       notas inativas ou concluídas
90 - Templates/      templates Obsidian
docs/                ADRs e documentação técnica                    ← board lê daqui
\`\`\`

## Board TUI

\`\`\`bash
# a partir deste diretório
npx obsidian-tasks-board board

# ou via Docker
docker run -it --rm -v \$(pwd):/vault pot-labs/otb
\`\`\`

## Sintaxe de tarefas

\`\`\`markdown
- [ ]  A fazer
- [/]  Em progresso
- [x]  Concluído
- [-]  Cancelado
\`\`\`

Campos inline suportados pelo board:
- \`[type:: técnico|estratégico]\`
- \`[refs:: PR#N, commit:hash]\`
- comentários: \`  - comment:: YYYY-MM-DD HH:mm @autor — texto\`
`;
}

// ── Public API ────────────────────────────────────────────────────────────────

/**
 * Scaffolds a new Obsidian vault directory with the otb board layout.
 * Returns the absolute path of the created vault root.
 */
export async function initVault(
  cwd: string,
  opts: InitOptions = {},
  log: (msg: string) => void = console.log,
): Promise<string> {
  const name   = opts.name ?? "my-vault";
  const dir    = opts.dir  ?? join(cwd, slug(name));
  const author = opts.author ?? "user";
  const force  = opts.force ?? false;
  const root   = resolve(dir);

  const projectTitle = "Primeiro Projeto";

  log(`\n🗂  Criando vault: ${name}`);
  log(`   Destino: ${root}\n`);

  // Directory structure
  const dirs = [
    ".obsidian",
    "00 - Inbox",
    "10 - Fleeting & Daily",
    "20 - Projects",
    "30 - Areas",
    "40 - Resources",
    "50 - Archives",
    "90 - Templates",
    "99 - Meta & Attachments",
    "docs",
  ];
  for (const d of dirs) await mkdirp(join(root, d));

  const w = (rel: string, content: string) =>
    writeIfNew(join(root, rel), content, force, log);

  // Obsidian config (minimal — user opens in Obsidian to finish setup)
  await w(".obsidian/app.json",     obsidianAppJson());
  await w(".obsidian/plugins.json", obsidianPluginsJson());

  // Board-facing files
  await w("20 - Projects/Board Global.md",           boardGlobalMd(name));
  await w(`20 - Projects/${projectTitle}.md`,        starterProjectMd(name, projectTitle));
  await w("90 - Templates/Template - Projeto.md",    projectTemplateMd(name));

  // Root files
  await w("README.md", readmeMd(name));

  log(`\n✅ Vault criado com sucesso!`);
  log(`\n📋 Próximos passos:`);
  log(`   1. Abra o vault no Obsidian: ${root}`);
  log(`   2. Instale os plugins: Tasks + Dataview`);
  log(`   3. Rode o board TUI: node --experimental-strip-types cli.ts board --vault ${root}`);
  log(`   4. Adicione projetos em 20 - Projects/ usando o template em 90 - Templates/`);
  log("");

  return root;
}
