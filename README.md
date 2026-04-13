# otb — Obsidian Tasks Board

> Terminal kanban board for [Obsidian](https://obsidian.md) vaults using the [Tasks plugin](https://obsidian-tasks-group.github.io/obsidian-tasks/).

Reads, visualizes and mutates `- [ ]` task lines **directly in Markdown files** — no database, no daemon, no sync layer. The vault stays the single source of truth.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  🗂  my-vault  12 tarefas                                                   │
├──────────────────────┬──────────────────────┬────────────┬──────────────────┤
│ ▶ ○ A Fazer (4)      │   ◐ Em Progresso (3) │ ● Feito(4) │   ✕ Cancelado(1)│
├──────────────────────┴──────────────────────┴────────────┴──────────────────┤
│ ▶ ○ Implementar autenticação JWT [type:: feat]                              │
│   ○ Revisar ADR-003 [refs:: #12]                                            │
│   ○ Escrever testes de integração                                           │
│   ○ Atualizar diagrama de arquitetura 💬2                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│  origem: 20 - Projects/backend.md:14   tipo: feat   refs: #12              │
└─────────────────────────────────────────────────────────────────────────────┘
  Tab/←→: col  ↑↓: item  i/t/d/x: mover  c: comentar  /: busca  q: sair
```

---

## Features

- **Kanban interativo** — 4 colunas: Todo / In Progress / Done / Cancelled
- **Mutações atômicas** — altera o checkbox no Markdown com verificação de fingerprint; nunca corrompe o arquivo
- **Comentários inline** — adiciona `comment::` subtópicos diretamente na nota
- **Filtros** — busca por texto e por projeto, com limpeza rápida
- **Painel de detalhes** — exibe origem, tipo, refs e comentários da tarefa selecionada
- **Auto-detecção de vault** — encontra `.obsidian/` no diretório atual ou em um filho direto
- **Binário estático único** — sem runtime, sem dependências externas; 3 MB, funciona em qualquer Linux x86-64
- **Scaffold de vault** — `otb init` cria toda a estrutura PARA + ADRs pronta para uso

---

## Instalação

### Pré-compilado (recomendado)

```bash
# Linux x86-64
curl -fsSL https://github.com/pot-labs/otb/releases/latest/download/otb -o ~/.local/bin/otb
chmod +x ~/.local/bin/otb
otb version
```

### Compilar do fonte

Requer **Go ≥ 1.23**.

```bash
git clone https://github.com/pot-labs/otb
cd otb
make build        # → bin/otb  (CGO_ENABLED=0, static, ~3 MB)
```

Para instalar no sistema:

```bash
cp bin/otb ~/.local/bin/otb   # ou outro diretório no $PATH
```

### Docker

```bash
docker build -f Dockerfile.test -t otb-tests .   # imagem de testes
docker build -t otb .                              # imagem de uso
```

---

## Desenvolvimento local

### Configuração inicial do repositório

Após clonar, instale os git hooks para que as validações de CI e segurança rodem automaticamente antes de cada `git push`:

```bash
make install-hooks
```

Isso copia `scripts/hooks/pre-push` para `.git/hooks/pre-push`.

> **Por que?** O hook espelha exatamente os workflows `ci.yml` e `security.yml` do GitHub Actions, prevenindo que código quebrado chegue ao remoto.

### Ferramentas opcionais (recomendadas)

Instale `staticcheck` e `govulncheck` para que o hook também rode os checks de segurança:

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

Se não estiverem presentes, o hook emite um aviso mas não bloqueia o push.

### O que o hook valida

Cada `git push` executa os seguintes passos em ordem:

| Step | Equivalente no CI |
|------|-------------------|
| `go vet ./...` | `ci.yml` + `security.yml` |
| `go test -race -count=1 -timeout=120s ./...` | `ci.yml` |
| `build` linux/amd64 (`CGO_ENABLED=0`) | `ci.yml` |
| `staticcheck ./...` | `security.yml` |
| `govulncheck ./...` | `security.yml` |

Se qualquer step falhar, o push é **abortado** com uma mensagem de erro indicando qual verificação falhou.

Para rodar as mesmas verificações manualmente a qualquer momento:

```bash
make vet      # go vet
make test     # testes com race detector
make build    # compilação estática
make lint     # staticcheck
make security # govulncheck
```

---

## Início rápido

```bash
# 1. Entre no diretório do vault (ou de um pai que contenha exatamente um vault)
cd ~/vaults/meu-vault

# 2. Abra o board
otb

# 3. Navegue com Tab/←→ entre colunas, ↑↓ entre tarefas
# 4. Pressione  i  para mover a tarefa para In Progress
# 5. Pressione  c  para adicionar um comentário
# 6. Pressione  q  para sair
```

---

## Uso

```
otb [board] [--vault <path>] [--project <nome>]
otb init    [--name <nome>]  [--dir <path>] [--author <handle>] [--force]
otb version
otb help
```

### Subcomandos

| Subcomando | Descrição |
|---|---|
| `board` (padrão) | Abre o TUI kanban interativo |
| `init` | Cria a estrutura de um novo vault Obsidian |
| `version` | Imprime a versão do binário |
| `help` | Mostra o help resumido |

### Flags — `board`

| Flag | Atalho | Descrição |
|---|---|---|
| `--vault <path>` | `-v` | Caminho explícito ao vault (auto-detectado se omitido) |
| `--project <nome>` | `-p` | Pré-filtra tarefas pelo nome do projeto |

### Flags — `init`

| Flag | Atalho | Padrão | Descrição |
|---|---|---|---|
| `--name <nome>` | `-n` | `my-vault` | Nome de exibição do vault |
| `--dir <path>` | `-d` | `./<slug(nome)>` | Diretório de destino |
| `--author <handle>` | `-a` | `user` | Handle do autor nos templates |
| `--force` | `-f` | `false` | Sobrescreve arquivos existentes |

---

## Sintaxe de tarefas suportada

O board lê qualquer arquivo `.md` dentro dos diretórios de scan.

```markdown
- [ ]  A fazer
- [/]  Em progresso
- [x]  Concluído   (também aceita [X])
- [-]  Cancelado
```

**Campos inline** (opcionais, stripped do texto de exibição):

```markdown
- [ ] Minha tarefa [type:: feat] [refs:: PR#42, commit:abc1234]
```

**Comentários** (subtópico indentado, lido e escrito pelo `otb`):

```markdown
- [ ] Minha tarefa
  - comment:: 2026-04-12 14:30 @alice — alinhamos o escopo com o cliente
  - comment:: 2026-04-13 09:00 @bob — iniciando implementação
```

---

## Detecção automática de vault

O `otb` resolve o vault na seguinte ordem:

1. `--vault <path>` explícito — validado e usado diretamente
2. O diretório atual contém `.obsidian/` — é o vault
3. Exatamente um filho do diretório atual contém `.obsidian/` — esse filho é o vault
4. Nenhum encontrado → erro com instruções

```bash
# Exemplos de auto-detecção
cd ~/vaults/meu-vault && otb          # caso 2
cd ~/vaults             && otb          # caso 3 (se só houver um vault)
otb --vault ~/vaults/meu-vault          # caso 1
```

---

## Estrutura de vault esperada

Por padrão o board varre dois diretórios:

```
<vault>/
├── .obsidian/              ← marcador de vault Obsidian (obrigatório)
├── 20 - Projects/          ← notas de projeto com listas de tarefas
└── docs/                   ← ADRs e documentação técnica com tarefas
```

Qualquer `.md` nesses diretórios que contenha linhas de tarefa é incluído.
Arquivos maiores que 10 MB e symlinks são ignorados silenciosamente.

---

## Cheatsheet de teclas

### Navegação

| Tecla | Ação |
|---|---|
| `Tab` / `→` | Próxima coluna |
| `Shift+Tab` / `←` | Coluna anterior |
| `↑` / `↓` | Navegar tarefas |

### Ações sobre tarefas

| Tecla | Ação |
|---|---|
| `i` | Mover → **In Progress** |
| `t` | Mover → **Todo** |
| `d` | Mover → **Done** |
| `x` | Mover → **Cancelled** |
| `c` | Adicionar comentário operacional |

> Uma confirmação (`s` / `Enter` para confirmar, qualquer outra tecla para cancelar) é exibida antes de cada mutação.

### Filtros

| Tecla | Ação |
|---|---|
| `/` ou `f` | Abrir filtro de texto (incremental) |
| `p` | Selecionar filtro de projeto |
| `F` | Limpar todos os filtros |
| `Esc` (com filtro ativo) | Limpar filtros e voltar ao board completo |

**Modo de filtro de texto** (após `/`):

| Tecla | Ação |
|---|---|
| Qualquer caractere | Adiciona ao termo de busca |
| `Backspace` | Remove o último caractere |
| `Enter` | Confirma e fecha a barra de filtro |
| `Esc` | Cancela e limpa o termo |

### Board

| Tecla | Ação |
|---|---|
| `r` | Recarregar vault do disco |
| `q` / `Esc` | Sair |

### Indicadores visuais

| Símbolo | Significado |
|---|---|
| `○` | Todo |
| `◐` | In Progress |
| `●` | Done |
| `✕` | Cancelled |
| `▶` | Tarefa selecionada |
| `💬N` | Tarefa com N comentários |
| `🔍` | Filtro ativo |

---

## Segurança

- **Mutações atômicas** — escrita em arquivo temporário no mesmo diretório + `rename(2)`. Nunca deixa o arquivo em estado parcial.
- **Verificação de fingerprint** — antes de alterar qualquer linha, verifica que o conteúdo não mudou desde a última leitura. Se o arquivo foi editado externamente, a operação falha com `ErrFingerprintMismatch` e pede reload.
- **Path traversal bloqueado** — `SourceFile` de tarefas é sempre relativo; caminhos absolutos e sequências `../` são rejeitados antes de qualquer I/O.
- **Sanitização de entrada** — todo texto lido do vault passa por `sanitize.ForDisplay` que remove sequências ANSI/VT, C0/C1, bidi overrides e null bytes antes de renderizar no terminal.
- **Binário sem CGO** — compilado com `CGO_ENABLED=0`; sem libc em runtime, superfície de ataque reduzida.

---

## Testes

```bash
# Suite completa (unit + race detector)
make test

# Testes de segurança black-box no binário
OTB_BINARY=./bin/otb go test ./internal/security/...

# Fuzz (30s por target)
make test-docker-fuzz

# Tudo em Docker (unit + security + fuzz)
make test-docker-full FUZZ_SECONDS=60
```

### Cobertura

| Pacote | Testes |
|---|---|
| `internal/parser` | `ParseTaskLine`, `ParseProjectFile` — status, campos inline, fingerprint, injeções ANSI/bidi/null/OSC |
| `internal/sanitize` | `ForDisplay` — CSI, OSC, DCS, C0/C1, bidi, null bytes, maxLength |
| `internal/reader` | Descoberta de arquivos, exclusões, symlinks, limites de tamanho |
| `internal/vault` | Auto-detect, ambiguidade, path traversal, caminhos explícitos |
| `internal/writer` | Mutação de status, comentários, fingerprint, escrita atômica, path traversal |
| `internal/security` | 13 testes black-box: injeção CLI, traversal de vault, sistema de dirs, RCE |

---

## Arquitetura

```
main.go                   CLI entry point — parse de flags, dispatch de subcomandos
cmd/
  board.go                TUI kanban (bubbletea Model/Update/View + estilos lipgloss)
  init.go                 Delegação para internal/init
internal/
  parser/   parser.go     Parseia linhas de tarefa e comentários do Markdown
  reader/   reader.go     Varre o vault, agrega tarefas por status (BoardData)
  writer/   writer.go     Mutações atômicas com fingerprint e guard de traversal
  vault/    resolver.go   Resolve e valida o caminho do vault
  sanitize/ sanitize.go   Remove sequências hostis de terminal e Unicode perigoso
  init/     init.go       Scaffold de estrutura de vault (PARA layout)
  security/ *_test.go     Testes black-box contra o binário compilado
tests/
  entrypoint.sh           Runner Docker: unit | fuzz | security | all
Dockerfile.test           Imagem multi-stage para execução dos testes
Makefile                  Targets: build, test, test-docker, test-docker-fuzz, …
```

### Dependências de runtime

| Módulo | Uso |
|---|---|
| `github.com/charmbracelet/bubbletea` | Loop de eventos e modelo TUI (Elm architecture) |
| `github.com/charmbracelet/lipgloss` | Estilos de terminal (cores, negrito, dim) |

Sem banco de dados, sem daemon, sem rede.

---

## `otb init` — estrutura gerada

```
<nome>/
├── .obsidian/
│   └── app.json
├── 00 - Inbox/
├── 10 - Fleeting & Daily/
├── 20 - Projects/
│   ├── Board Global.md          Vista kanban global (Dataview)
│   └── Primeiro Projeto.md      Projeto de exemplo com tarefas
├── 30 - Areas/
├── 40 - Resources/
├── 50 - Archives/
├── 90 - Templates/
│   └── Template - Projeto.md    Template reutilizável de projeto
├── 99 - Meta & Attachments/
├── docs/                        ADRs e documentação técnica
└── README.md
```

Após o `init`, abra o vault no Obsidian e instale os plugins **Tasks** e **Dataview** para funcionalidade completa dentro do Obsidian.

