# Keybindings — otb

## Navegação

| Tecla | Ação |
|---|---|
| `Tab` / `→` | Próxima coluna |
| `Shift+Tab` / `←` | Coluna anterior |
| `↑` | Mover seleção para cima |
| `↓` | Mover seleção para baixo |

## Ações sobre tarefas

| Tecla | Ação |
|---|---|
| `i` | Mover tarefa selecionada → **In Progress** |
| `t` | Mover tarefa selecionada → **Todo** |
| `d` | Mover tarefa selecionada → **Done** |
| `x` | Mover tarefa selecionada → **Cancelled** |
| `c` | Adicionar comentário operacional |

### Confirmação de mutação

Antes de qualquer mudança de status uma barra de confirmação é exibida:

| Tecla | Ação |
|---|---|
| `s` / `y` / `Enter` | Confirmar mudança |
| Qualquer outra tecla | Cancelar |

### Modo de comentário (após `c`)

| Tecla | Ação |
|---|---|
| Qualquer caractere | Adiciona ao texto do comentário |
| `Backspace` | Remove o último caractere |
| `Enter` | Confirma e salva o comentário |
| `Esc` | Cancela sem salvar |

## Filtros

| Tecla | Ação |
|---|---|
| `/` ou `f` | Abrir filtro de texto (incremental) |
| `p` | Abrir seletor de projeto |
| `F` | Limpar todos os filtros ativos |
| `Esc` (com filtro ativo) | Limpar filtros e retornar ao board completo |

### Modo de filtro de texto (após `/`)

| Tecla | Ação |
|---|---|
| Qualquer caractere | Adiciona ao termo de busca |
| `Backspace` | Remove o último caractere |
| `Enter` | Confirma e fecha a barra de filtro |
| `Esc` | Cancela e limpa o termo |

### Modo de seleção de projeto (após `p`)

| Tecla | Ação |
|---|---|
| `↑` / `↓` | Navegar projetos |
| `Enter` | Selecionar projeto (ou "Todos") |
| `Esc` / `q` | Fechar sem alterar filtro |

## Board

| Tecla | Ação |
|---|---|
| `r` | Recarregar board do disco |
| `q` / `Esc` | Sair (sem filtro ativo) |
| `ctrl+c` | Forçar saída |

## Indicadores visuais

| Símbolo | Significado |
|---|---|
| `○` | Todo |
| `◐` | In Progress |
| `●` | Done |
| `✕` | Cancelled |
| `▶` (ciano) | Tarefa selecionada |
| `💬N` | Tarefa com N comentários |
| `🔍` | Filtro de texto ou projeto ativo |
| `… N mais` | Tarefas além das 12 visíveis (rolar com ↑↓) |
