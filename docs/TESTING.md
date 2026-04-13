# Testes — otb

## Estrutura

```
internal/
├── parser/
│   ├── parser_test.go       testes unitários
│   └── fuzz_test.go         fuzz targets
├── reader/
│   ├── reader_test.go
│   └── (sem fuzz — I/O coberto pelos testes unitários)
├── sanitize/
│   ├── sanitize_test.go
│   └── fuzz_test.go
├── vault/
│   ├── resolver_test.go
│   └── fuzz_test.go
├── writer/
│   ├── writer_test.go
│   └── fuzz_test.go
├── security/
│   └── security_test.go     testes black-box no binário compilado
└── testutil/
    └── testutil.go          helpers compartilhados (MakeVault, etc.)
tests/
└── entrypoint.sh            runner Docker com modos unit/fuzz/security/all
Dockerfile.test              imagem multi-stage para execução isolada
```

---

## Rodar localmente

```bash
# Todos os testes unitários + race detector
make test

# Testes de segurança black-box (requer bin/otb compilado)
make build
OTB_BINARY=./bin/otb go test -v ./internal/security/...

# Um fuzz target específico (30 segundos)
go test ./internal/parser/... -run='^$' -fuzz='^FuzzParseTaskLine$' -fuzztime=30s

# Todos os fuzz targets (5s cada, para validação rápida)
for pkg in parser sanitize vault writer; do
  go test ./internal/$pkg/... -run='^$' -fuzz='^Fuzz' -fuzztime=5s
done
```

---

## Docker

A imagem `Dockerfile.test` executa os testes em ambiente isolado:
- Usuário não-root (uid 1000)
- Filesystem read-only (`--read-only`)
- Escrita apenas em `/tmp` e `/root/.cache` (via `--tmpfs`)
- `OTB_BINARY=/app/bin/otb` configurado automaticamente

```bash
# Build da imagem de testes
make test-docker-build

# Unit + security
make test-docker

# Fuzz (30s por target)
make test-docker-fuzz

# Fuzz com duração customizada
make test-docker-fuzz FUZZ_SECONDS=120

# Tudo (unit + fuzz + security)
make test-docker-full
```

Resultados são gravados em `./test-results/`:

```
test-results/
├── unit.log
├── security.log
├── fuzz-FuzzParseTaskLine.log
├── fuzz-FuzzParseProjectFile.log
├── fuzz-FuzzForDisplay.log
├── fuzz-FuzzChangeTaskStatus.log
├── fuzz-FuzzAddTaskComment.log
├── fuzz-FuzzChangeTaskStatus_SourceFilePath.log
├── fuzz-FuzzVaultResolve.log
└── fuzz-FuzzIsUnderVault.log
```

---

## Fuzz targets

| Target | Pacote | O que busca |
|---|---|---|
| `FuzzParseTaskLine` | `parser` | panic, Text não-UTF8, ESC/C0/C1 no output, status inválido |
| `FuzzParseProjectFile` | `parser` | same + `Comments.LineNumber ≤ task.LineNumber` |
| `FuzzForDisplay` | `sanitize` | panic, saída com ESC/C0/C1/bidi, violação de `maxLength` |
| `FuzzChangeTaskStatus` | `writer` | panic, arquivo corrompido, crescimento explosivo |
| `FuzzAddTaskComment` | `writer` | panic, ESC escrito no disco, injeção em `author` |
| `FuzzChangeTaskStatus_SourceFilePath` | `writer` | path traversal via `SourceFile` arbitrário |
| `FuzzVaultResolve` | `vault` | vault resolvido em path de sistema |
| `FuzzIsUnderVault` | `vault` | `relPath` absoluto aceito, escape do vault root |

### Interpretar resultados do fuzz

Quando um fuzz target encontra um *failing input*, o Go salva o corpus em:

```
internal/<pkg>/testdata/fuzz/<FuncName>/<hash>
```

Para reproduzir:

```bash
go test ./internal/<pkg>/... -run='^<FuncName>/<hash>$'
```

Para adicionar ao corpus permanente:

```bash
# Confirme que o bug foi corrigido, depois:
go test ./internal/<pkg>/... -run='^$' -fuzz='^<FuncName>$' -fuzztime=1s
# O novo corpus sobrevive entre runs automaticamente
```

---

## Testes de segurança black-box

`internal/security/security_test.go` executa o binário compilado com `exec.Command` e verifica invariantes externas:

| Teste | Categoria | O que verifica |
|---|---|---|
| `TestBinary_Version` | smoke | binário executa e imprime versão |
| `TestBinary_Help` | smoke | help contém keywords esperadas |
| `TestBinary_VaultTraversal_RelativeDotDot` | traversal | `../../etc` é rejeitado |
| `TestBinary_VaultPath_SystemDirs` | traversal | `/etc`, `/bin`, `/`, `/proc`, `/sys`, `/usr/bin` são rejeitados |
| `TestBinary_VaultPath_NonExistent` | input | path inexistente retorna erro limpo |
| `TestBinary_VaultPath_InjectionPayloads` | injection | `$(...)`, backticks, ANSI em `--vault` não executam código |
| `TestBinary_ProjectFlag_Injection` | injection | shell injection em `--project` não executa código |
| `TestBinary_Init_DirTraversal` | traversal | `../` em `--dir` não escapa do sandbox |
| `TestBinary_Init_AbsoluteSystemDir` | traversal | `/etc/` como `--dir` não cria arquivos |
| `TestBinary_Init_NameInjection` | injection | shell injection em `--name` não executa código |
| `TestBinary_UnknownArgs` | robustez | flags desconhecidas não travam nem crasheiam |
| `TestBinary_Board_NoVaultAutoDetect` | erro | sem vault → saída limpa com código ≠ 0 |
| `TestBinary_NoPathLeakInError` | info leak | stderr não contém paths de build (`.go:`) |

### Invariante `assertCleanExit`

Um teste falha se o processo morrer com:
- `signal:` no erro → indicativo de panic/SIGSEGV
- `timeout` → binário travado (hang)

Exit code ≠ 0 **não** falha — erros de input são esperados e corretos.

---

## Variáveis de ambiente dos testes

| Variável | Padrão | Descrição |
|---|---|---|
| `OTB_BINARY` | auto (runtime.Caller) | Caminho ao binário para testes de segurança |
| `TEST_MODE` | `unit` | Modo do runner Docker: `unit`, `fuzz`, `security`, `all`, `fuzz+security` |
| `FUZZ_SECONDS` | `30` | Duração de cada fuzz target (segundos) |
| `TEST_TIMEOUT` | `120` | Timeout dos testes unitários (segundos) |
