# Arquitetura — otb

## Visão geral

`otb` é um binário Go estático que implementa um TUI kanban para vaults Obsidian. Não há daemon, banco de dados ou sincronização — todo estado vive nos arquivos Markdown do vault.

```
┌──────────────┐      lê/escreve       ┌──────────────────────┐
│  Terminal    │ ◄──── bubbletea ────► │  Arquivos Markdown   │
│  (usuário)   │                        │  no vault Obsidian   │
└──────────────┘                        └──────────────────────┘
       │
       ▼
  main.go (CLI)
       │
  ┌────┴─────────────┐
  │    cmd/board.go  │  ← TUI Model (Elm architecture)
  │    cmd/init.go   │  ← Scaffold de vault
  └────┬─────────────┘
       │
  ┌────┴──────────────────────────────────────┐
  │  internal/                                │
  │  ├── vault/resolver.go   detecção de vault│
  │  ├── reader/reader.go    scan + parse      │
  │  ├── parser/parser.go    parseia tarefas   │
  │  ├── writer/writer.go    mutações atômicas │
  │  ├── sanitize/sanitize.go  proteção TUI   │
  │  └── init/init.go        scaffold         │
  └───────────────────────────────────────────┘
```

---

## Pacotes

### `internal/vault` — Resolução de vault

**Entrada:** caminho explícito (string) ou cwd  
**Saída:** caminho canônico absoluto e validado

Ordem de detecção:
1. `--vault <path>` explícito → `filepath.EvalSymlinks` + checar `.obsidian/`
2. cwd contém `.obsidian/`
3. Exatamente um filho do cwd contém `.obsidian/`
4. `ErrNotFound` ou `ErrAmbiguous`

Guards:
- `isSensitivePath`: bloqueia `/`, `/etc`, `/bin`, `/sbin`, `/lib`, `/proc`, `/sys`
- `IsUnderVault`: rejeita `relPath` absolutos e sequências `../` que escapam do vault root

---

### `internal/parser` — Parseia linhas de tarefa

Reconhece a sintaxe do Tasks plugin:

```
- [ ]  Todo        (qualquer char que não seja /, x, X, -)
- [/]  In Progress
- [x]  Done
- [-]  Cancelled
```

Extrai campos inline via regex:

```
\[type:: <valor>\]   →  Task.Type
\[refs:: <valor>\]   →  Task.Refs
```

Lê comentários como subtópicos indentados:

```
  - comment:: <timestamp> @<autor> — <texto>
```

**Fingerprint:** `"<sourceFile>:L<line>:<normalizedText>"` — identidade estável de uma tarefa para detecção de conflito de escrita.

Limites (em runes): `Text=300`, `Type=100`, `Refs=200`, `Comment=200`.

---

### `internal/reader` — Varre o vault

```go
type Config struct {
    ScanDirs []string            // padrão: ["20 - Projects", "docs"]
    Excluded map[string]struct{} // padrão: {"Board Global.md"}
}

type BoardData struct {
    Projects []ProjectSummary
    AllTasks []parser.Task
    ByStatus map[parser.TaskStatus][]parser.Task
}
```

Guards:
- Symlinks são ignorados (`os.Lstat` + `ModeSymlink`)
- Arquivos > 10 MB são ignorados
- Diretórios inexistentes são silenciados (não falham o scan)

---

### `internal/writer` — Mutações atômicas

**Modelo de segurança (em ordem):**

1. **Path traversal check** — `vault.IsUnderVault` valida `task.SourceFile`
2. **Re-read** — lê o arquivo do disco imediatamente antes de escrever
3. **Fingerprint verify** — confirma que a linha ainda bate com `task.Text` normalizado
4. **Mutação mínima** — substitui apenas o char do checkbox (ou insere uma linha de comentário)
5. **Atomic write** — escreve em `.tmp-otb-<random>.md` no mesmo diretório, depois `os.Rename`

Se `ErrFingerprintMismatch` é retornado, a TUI exibe mensagem pedindo `r` (reload) antes de tentar novamente.

**Preservação de EOL:** detecta `\r\n` vs `\n` na leitura e preserva ao escrever.

---

### `internal/sanitize` — Proteção de terminal

Remove antes de renderizar ou escrever no vault:

| Categoria | Exemplos |
|---|---|
| CSI sequences | `\x1b[31m` (cor), `\x1b[2J` (limpar tela) |
| OSC sequences | `\x1b]0;título\x07` (sobrescrever título do terminal) |
| DCS/SOS/PM/APC | sequências de estado de terminal |
| SS2/SS3 | `\x1bN`, `\x1bO` |
| RIS | `\x1bc` (reset total do terminal) |
| C0 controls | `\x00`–`\x1F` (null, BEL, BS, etc.) |
| C1 controls | `\x80`–`\x9F` (equivalentes 8-bit de CSI/OSC) |
| Bidi overrides | `U+202A`–`U+202E`, `U+2066`–`U+2069`, `U+200E/F`, `U+FEFF` |

Após remoção: colapsa whitespace, trunca em `maxLength` runes (com `…`).

---

### `cmd/board.go` — TUI (bubbletea)

Segue a **Elm architecture** do bubbletea:

```
boardModel  →  Init()   → tea.Cmd
            →  Update() → (tea.Model, tea.Cmd)
            →  View()   → string
```

**Modos (`boardMode`):**

| Modo | Ativado por | Comportamento |
|---|---|---|
| `modeNormal` | padrão | navegação e dispatch de ações |
| `modeFilterInput` | `/` ou `f` | captura texto de filtro |
| `modeConfirm` | `i`/`t`/`d`/`x` | aguarda confirmação antes de mutar |
| `modeComment` | `c` | captura texto do comentário |
| `modeProjectSelect` | `p` | lista de projetos para filtro |

**Mensagens assíncronas:**

| Mensagem | Origem | Efeito |
|---|---|---|
| `reloadMsg` | `reader.Read` | substitui `m.data`, clamp de índice |
| `errMsg` | writer ou reader | exibe na barra de status |
| `tea.KeyMsg` | teclado | dispatch para handler de modo |

---

## Fluxo de mutação de status

```
usuário pressiona "i"
        │
        ▼
handleNormal → startMove(StatusInProgress)
        │
        ▼
modeConfirm — usuário confirma com "s"
        │
        ▼
handleConfirm → m.moveCmd(task, newStatus)   ← tea.Cmd (goroutine)
        │
        ▼ (goroutine)
writer.ChangeTaskStatus(vaultPath, task, newStatus)
  1. vault.IsUnderVault(vaultPath, task.SourceFile)
  2. readLines(filePath)
  3. verifyLine(lines, task)
  4. reCheckbox.ReplaceAllStringFunc(lines[idx], ...)
  5. writeLines(filePath, lines, eol)   ← atomic rename
        │
        ▼
reloadMsg{data}  →  Update()  →  m.data = msg.data
```

---

## Build

```bash
# Binário estático linux/amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o bin/otb .

# Cross-compile linux/arm64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -ldflags="-s -w" -o bin/otb-arm64 .
```

Flags de segurança:

| Flag | Efeito |
|---|---|
| `CGO_ENABLED=0` | binário totalmente estático, sem libc |
| `-trimpath` | remove caminhos do fonte do binário (sem leak de paths) |
| `-ldflags="-s -w"` | strip de symbol table e DWARF (menor e sem paths de debug) |
