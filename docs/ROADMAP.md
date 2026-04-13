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

### 2. Detalhe da tarefa (visualização do Markdown)

Comando para abrir o contexto completo de uma tarefa — lê o arquivo Markdown de origem e renderiza o conteúdo na TUI.

- Atalho para abrir painel de detalhe (ex.: `Enter` ou `d`)
- Leitura e renderização do Markdown do arquivo fonte
- Exibição do trecho ao redor da tarefa selecionada
- Navegação de volta ao board sem perder posição
