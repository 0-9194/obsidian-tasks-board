// Package initvault scaffolds a new Obsidian vault with the otb default layout.
package initvault

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options for the init command.
type Options struct {
	Name   string // vault display name (default: "my-vault")
	Dir    string // target directory (default: ./<slug(name)>)
	Author string // author handle used in templates (default: "user")
	Force  bool   // overwrite existing files
}

func slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	s := b.String()
	// collapse consecutive dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func today() string {
	return time.Now().Format("2006-01-02")
}

// Run scaffolds the vault. Returns the absolute path of the created root.
func Run(cwd string, opts Options, w io.Writer) (string, error) {
	if opts.Name == "" {
		opts.Name = "my-vault"
	}
	if opts.Dir == "" {
		opts.Dir = filepath.Join(cwd, slug(opts.Name))
	}
	if opts.Author == "" {
		opts.Author = "user"
	}

	root, err := filepath.Abs(opts.Dir)
	if err != nil {
		return "", fmt.Errorf("invalid directory: %w", err)
	}

	fmt.Fprintf(w, "\n🗂  Criando vault: %s\n", opts.Name)
	fmt.Fprintf(w, "   Destino: %s\n\n", root)

	dirs := []string{
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
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0750); err != nil {
			return "", fmt.Errorf("creating directory %q: %w", d, err)
		}
	}

	files := map[string]string{
		".obsidian/app.json":                            obsidianAppJSON(),
		".obsidian/plugins.json":                        "{}",
		"20 - Projects/Board Global.md":                 boardGlobalMD(opts.Name),
		"20 - Projects/Primeiro Projeto.md":             starterProjectMD(opts.Name, "Primeiro Projeto"),
		"90 - Templates/Template - Projeto.md":          projectTemplateMD(),
		"README.md":                                     readmeMD(opts.Name),
		".gitignore":                                   gitignoreContent(),
	}

	for rel, content := range files {
		dest := filepath.Join(root, rel)
		if err := writeIfNew(dest, content, opts.Force, w); err != nil {
			return "", err
		}
	}

	fmt.Fprintln(w, "\n✅ Vault criado com sucesso!")
	fmt.Fprintln(w, "\n📋 Próximos passos:")
	fmt.Fprintf(w, "   1. Abra o vault no Obsidian: %s\n", root)
	fmt.Fprintln(w, "   2. Instale os plugins: Tasks + Dataview")
	fmt.Fprintf(w, "   3. Rode o board TUI: otb board --vault %s\n", root)
	fmt.Fprintln(w, "   4. Adicione projetos em '20 - Projects/' usando o template em '90 - Templates/'")
	fmt.Fprintln(w, "")

	return root, nil
}

func writeIfNew(path, content string, force bool, w io.Writer) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(w, "  skip   %s (already exists)\n", path)
			return nil
		}
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}
	fmt.Fprintf(w, "  create %s\n", path)
	return nil
}

// ── File templates ────────────────────────────────────────────────────────────

func obsidianAppJSON() string {
	return `{
  "legacyEditorEnabled": false,
  "livePreview": true
}
`
}

func boardGlobalMD(name string) string {
	d := today()
	return fmt.Sprintf(`---
title: Board Global
aliases:
  - Board
  - Kanban Global
tags:
  - board
status: active
created: %s
updated: %s
category: board
---

# 🗂️ Board Global — %s

> View dinâmica de todas as tarefas dos projetos e documentação.
> **Não edite tarefas aqui** — vá até a nota de origem.

---

> [!todo]+ 📋 A Fazer
>
> `+"```"+"tasks"+`
> not done
> filter by function task.status.symbol === ' '
> (path includes 20 - Projects) OR (path includes docs)
> group by filename
> sort by priority
> `+"```"+`

> [!warning]+ 🔄 Em Progresso
>
> `+"```"+"tasks"+`
> not done
> filter by function task.status.symbol === '/'
> (path includes 20 - Projects) OR (path includes docs)
> group by filename
> sort by priority
> `+"```"+`

> [!success]+ ✅ Concluído
>
> `+"```"+"tasks"+`
> done
> (path includes 20 - Projects) OR (path includes docs)
> group by filename
> sort by done date reverse
> `+"```"+`

> [!failure]+ ✕ Cancelado
>
> `+"```"+"tasks"+`
> not done
> filter by function task.status.symbol === '-'
> (path includes 20 - Projects) OR (path includes docs)
> group by filename
> `+"```"+`
`, d, d, name)
}

func starterProjectMD(vaultName, projectTitle string) string {
	d := today()
	return fmt.Sprintf(`---
title: "%s"
tags:
  - projeto
  - status/em-progresso
status: em-progresso
created: %s
updated: %s
type: técnico
---

# 🚀 %s

## 🎯 Objetivo

_Descreva o objetivo deste projeto aqui._

---

## 📋 Board do Projeto

> [!todo]- 📋 A Fazer
> `+"```"+"tasks"+`
> not done
> filter by function task.status.symbol === ' '
> path includes 20 - Projects/%s
> sort by priority
> `+"```"+`

> [!warning]- 🔄 Em Progresso
> `+"```"+"tasks"+`
> not done
> filter by function task.status.symbol === '/'
> path includes 20 - Projects/%s
> sort by priority
> `+"```"+`

> [!success]- ✅ Concluído
> `+"```"+"tasks"+`
> done
> path includes 20 - Projects/%s
> sort by done date reverse
> `+"```"+`

---

## 📝 Tarefas

- [ ] Primeira tarefa [type:: técnico]
- [ ] Segunda tarefa [type:: estratégico]
- [/] Tarefa em progresso [type:: técnico]
`, projectTitle, d, d, projectTitle, projectTitle, projectTitle, projectTitle)
}

func projectTemplateMD() string {
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

---

## 📝 Tarefas

- [ ] 

`
}

func readmeMD(name string) string {
	return fmt.Sprintf(`# %s

Vault Obsidian gerenciado com [otb](https://github.com/pot-labs/otb).

## Estrutura

`+"```"+`
00 - Inbox/          rascunhos e capturas não processadas
10 - Fleeting/       notas diárias e efêmeras
20 - Projects/       projetos ativos com board de tarefas embutido  ← board lê daqui
30 - Areas/          responsabilidades contínuas
40 - Resources/      conhecimento atômico e guias de referência
50 - Archives/       notas inativas ou concluídas
90 - Templates/      templates Obsidian
docs/                ADRs e documentação técnica                    ← board lê daqui
`+"```"+`

## Board TUI

`+"```"+"bash"+`
# a partir deste diretório
otb board

# com vault explícito
otb board --vault /path/to/vault
`+"```"+`

## Sintaxe de tarefas

`+"```"+"markdown"+`
- [ ]  A fazer
- [/]  Em progresso
- [x]  Concluído
- [-]  Cancelado
`+"```"+`
`, name)
}

func gitignoreContent() string {
	return `# ===================================================================
# .gitignore para Vaults de Obsidian & Foam
# ===================================================================

# Arquivos de ambiente
.env
.env.local
.env.development.local
.env.test.local
.env.production.local

# Dependências
node_modules/

# Arquivos de Sistema e temporários
*~
.DS_Store
Thumbs.db
*.log
.gemini/
# Ignorar o restante do diretório .vscode/, por isso o *
.vscode/*
# Exceção para .vscode/extensions.json
!.vscode/extensions.json

# Pasta de lixo do Obsidian
.trash/

# ===================================================================
# Arquivos específicos do usuário - NUNCA rastrear
# ===================================================================
# Cache, estado do workspace, histórico, etc.
.obsidian/cache
.obsidian/workspace.json
.obsidian/workspaces.json
.obsidian/workspace-mobile.json
.obsidian/user.json
.obsidian/graph.json
# Ignora index do Plugin Copilot
.obsidian/copilot-index-*.json
.copilot-index/
# Ignora index do Plugin Smart Composer
.smtcmp_json_db/.initial_migration_completed
.smtcmp_vector_db.tar.gz

# ===================================================================
# Arquivos de configuração de Plugins do Obsidian
# ===================================================================
# A pasta de plugins da comunidade é ignorada para evitar conflitos
# de sincronização entre diferentes dispositivos e sistemas.
# Atualizações de plugins ou mudanças de configuração em um local
# podem quebrar a instalação em outro.
#
# A lista de plugins ativados (.obsidian/community-plugins.json) É
# rastreada, permitindo que cada ambiente instale as mesmas
# versões de forma independente e segura.
#
# Consulte a documentação em 'docs/estrategia-plugins-obsidian.md'
# para mais detalhes.
.obsidian/plugins/

# Arquivos de configuração que podem conter chaves de API ou dados sensíveis.
obsidian-web-clipper-settings.json

# ===================================================================
# Arquivos de configuração do Vault - Rastrear por padrão
# ===================================================================
# As linhas abaixo estão comentadas porque os arquivos não existem
# ainda. Descomente se você criar e quiser ignorá-los.
#
# .obsidian/hotkeys.json
# .obsidian/community-plugins.json
# .obsidian/publish.json

# ===================================================================
# Específico do Foam (para VS Code)
# ===================================================================
.foam/

gha-creds-*.json
`
}
