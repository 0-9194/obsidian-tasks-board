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

### ✅ 2. Detalhe da tarefa (visualização do Markdown) — `v0.1.2`

Comando para abrir o contexto completo de uma tarefa — lê o arquivo Markdown de origem e renderiza o conteúdo na TUI.

- ✅ `Enter` abre painel de detalhe da tarefa selecionada (`modeDetail`)
- ✅ Leitura do arquivo fonte e exibição de ±8 linhas de contexto ao redor da tarefa
- ✅ Linha da tarefa destacada com `▶` e numeração de linhas
- ✅ Scroll com `↑↓` / `j/k` e indicador de percentual
- ✅ Metadados completos: arquivo, linha, tipo, refs, status, comentários
- ✅ `Esc` / `q` / `Enter` voltam ao board sem perder posição

---

## 🔜 Próximas features

### ✅ 1. Troca de estado das tarefas

Permitir que o usuário altere o status de uma tarefa diretamente na TUI (ex.: `[ ]` → `[/]` → `[x]` → `[-]`) sem precisar abrir o arquivo manualmente.

- ✅ Teclas `i/t/d/x/b` movem a tarefa para Em Progresso / A Fazer / Concluído / Cancelado / Backlog
- ✅ Confirmação `[s/N]` antes de aplicar a mudança
- ✅ Escrita atômica de volta no arquivo Markdown de origem (via `changeTaskStatus`)
- ✅ Coluna Backlog adicionada ao board com ícone `◌` e tecla `b`
- ✅ Confirmação visual do estado atualizado via `statusMsg`

---

### 3. Corpo/descrição da tarefa

Exibir o **conteúdo Markdown** de uma tarefa no painel de detalhe — o trecho do arquivo fonte entre a linha da task e a próxima task ou heading é o "corpo" da tarefa.

**Motivação:** Tasks no Obsidian frequentemente têm descrição narrativa escrita logo abaixo da linha `- [ ] ...` (parágrafos, listas, links). O painel de detalhe já lê o arquivo — basta delimitar e exibir esse trecho como corpo.

**Escopo sugerido:**

- No painel de detalhe (`modeDetail`): após a linha da task, capturar tudo até a próxima task ou heading como corpo
- Renderizar esse conteúdo com destaque visual separado do restante do contexto do arquivo
- Scroll cobre task + corpo naturalmente

**Critérios de aceite:**
- [ ] Body visível no painel de detalhe ao pressionar `Enter` na task
- [ ] Tasks sem conteúdo abaixo não quebram o layout existente
