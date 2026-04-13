# Roadmap — obsidian-tasks-board (otb)

Funcionalidades planejadas para versões futuras.

---

## ✅ Concluído

### 3. Board com lanes visuais (ao invés de tabs) — `v0.1.1`

Substituir (ou complementar) a navegação por abas com uma representação kanban de colunas visíveis simultaneamente na tela.

- ✅ Layout horizontal com lanes: **A Fazer | Em Progresso | Concluído | Cancelado**
- ✅ Navegação entre lanes com `←` / `→`
- ✅ Redimensionamento automático das colunas conforme o terminal
- ✅ Compatível com o filtro por projeto já existente

---

## 🔜 Próximas features

### 1. Troca de estado das tarefas

Permitir que o usuário altere o status de uma tarefa diretamente na TUI (ex.: `[ ]` → `[/]` → `[x]` → `[-]`) sem precisar abrir o arquivo manualmente.

- Atalho de teclado para ciclar entre estados
- Escrita de volta no arquivo Markdown de origem (aproveitando o `writer` já existente)
- Confirmação visual do estado atualizado no board

---

### ✅ 2. Detalhe da tarefa (visualização do Markdown) — `v0.1.2`

Comando para abrir o contexto completo de uma tarefa — lê o arquivo Markdown de origem e renderiza o conteúdo na TUI.

- ✅ `Enter` abre painel de detalhe da tarefa selecionada (`modeDetail`)
- ✅ Leitura do arquivo fonte e exibição de ±8 linhas de contexto ao redor da tarefa
- ✅ Linha da tarefa destacada com `▶` e numeração de linhas
- ✅ Scroll com `↑↓` / `j/k` e indicador de percentual
- ✅ Metadados completos: arquivo, linha, tipo, refs, status, comentários
- ✅ `Esc` / `q` / `Enter` voltam ao board sem perder posição
