// Package cmd contains the subcommand implementations for otb.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pot-labs/otb/internal/parser"
	"github.com/pot-labs/otb/internal/reader"
	"github.com/pot-labs/otb/internal/vault"
	"github.com/pot-labs/otb/internal/writer"
)

// BoardFlags holds parsed CLI flags for the board subcommand.
type BoardFlags struct {
	VaultPath string
	Project   string
}

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleYellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleDim     = lipgloss.NewStyle().Faint(true)
	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	styleSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

// ── Column definitions ────────────────────────────────────────────────────────

type column struct {
	id    parser.TaskStatus
	label string
	icon  string
}

var columns = []column{
	{parser.StatusTodo, "A Fazer", "○"},
	{parser.StatusInProgress, "Em Progresso", "◐"},
	{parser.StatusDone, "Concluído", "●"},
	{parser.StatusCancelled, "Cancelado", "✕"},
}

const maxVisible = 12

// ── Board state ───────────────────────────────────────────────────────────────

type boardMode int

const (
	modeNormal boardMode = iota
	modeFilterInput
	modeConfirm
	modeComment
	modeProjectSelect
)

type statusMsg struct {
	text string
	ok   bool
}

type boardModel struct {
	data          *reader.BoardData
	vaultPath     string
	boardTitle    string
	colIdx        int
	taskIdx       int
	filterText    string
	filterProject string
	mode          boardMode
	confirmTarget parser.TaskStatus
	commentDraft  string
	projectList   []string
	projectCursor int
	status        *statusMsg
	err           error
	width         int
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m *boardModel) currentColumn() column {
	return columns[m.colIdx]
}

func (m *boardModel) filteredTasks(status parser.TaskStatus) []parser.Task {
	tasks := m.data.ByStatus[status]
	if m.filterProject != "" {
		var filtered []parser.Task
		for _, t := range tasks {
			if strings.Contains(t.SourceFile, m.filterProject) {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}
	needle := strings.ToLower(strings.TrimSpace(m.filterText))
	if needle != "" {
		var filtered []parser.Task
		for _, t := range tasks {
			if strings.Contains(strings.ToLower(t.Text), needle) ||
				strings.Contains(strings.ToLower(t.Type), needle) {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}
	return tasks
}

func (m *boardModel) currentTasks() []parser.Task {
	return m.filteredTasks(m.currentColumn().id)
}

func (m *boardModel) selectedTask() *parser.Task {
	tasks := m.currentTasks()
	if m.taskIdx >= 0 && m.taskIdx < len(tasks) {
		t := tasks[m.taskIdx]
		return &t
	}
	return nil
}

func (m *boardModel) hasFilter() bool {
	return m.filterProject != "" || strings.TrimSpace(m.filterText) != ""
}

func (m *boardModel) clampTaskIdx() {
	tasks := m.currentTasks()
	if len(tasks) == 0 {
		m.taskIdx = 0
		return
	}
	if m.taskIdx >= len(tasks) {
		m.taskIdx = len(tasks) - 1
	}
	if m.taskIdx < 0 {
		m.taskIdx = 0
	}
}

// ── Messages ──────────────────────────────────────────────────────────────────

type reloadMsg struct{ data *reader.BoardData }
type errMsg struct{ err error }
type statusClearMsg struct{}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m boardModel) Init() tea.Cmd {
	return nil
}

// ── Reload command ────────────────────────────────────────────────────────────

func (m *boardModel) reloadCmd() tea.Cmd {
	vp := m.vaultPath
	return func() tea.Msg {
		data, err := reader.Read(vp, nil)
		if err != nil {
			return errMsg{err}
		}
		return reloadMsg{data}
	}
}

// ── Move command ──────────────────────────────────────────────────────────────

func (m *boardModel) moveCmd(task *parser.Task, newStatus parser.TaskStatus) tea.Cmd {
	vp := m.vaultPath
	t := *task
	return func() tea.Msg {
		if err := writer.ChangeTaskStatus(vp, &t, newStatus); err != nil {
			return errMsg{err}
		}
		data, err := reader.Read(vp, nil)
		if err != nil {
			return errMsg{err}
		}
		return reloadMsg{data}
	}
}

// ── Comment command ───────────────────────────────────────────────────────────

func (m *boardModel) commentCmd(task *parser.Task, text string) tea.Cmd {
	vp := m.vaultPath
	t := *task
	return func() tea.Msg {
		if err := writer.AddTaskComment(vp, &t, text, "user"); err != nil {
			return errMsg{err}
		}
		data, err := reader.Read(vp, nil)
		if err != nil {
			return errMsg{err}
		}
		return reloadMsg{data}
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m boardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case reloadMsg:
		m.data = msg.data
		m.clampTaskIdx()
		return m, nil

	case errMsg:
		errText := msg.err.Error()
		if errors.Is(msg.err, writer.ErrFingerprintMismatch) {
			errText = "Arquivo mudou — recarregue (r) e tente novamente."
		}
		m.status = &statusMsg{text: errText, ok: false}
		m.mode = modeNormal
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m boardModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeFilterInput:
		return m.handleFilterInput(msg)
	case modeConfirm:
		return m.handleConfirm(msg)
	case modeComment:
		return m.handleComment(msg)
	case modeProjectSelect:
		return m.handleProjectSelect(msg)
	}
	return m.handleNormal(msg)
}

func (m boardModel) handleNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc":
		if m.hasFilter() {
			m.filterProject = ""
			m.filterText = ""
			m.taskIdx = 0
			m.status = &statusMsg{"Filtros removidos", true}
		} else {
			return m, tea.Quit
		}

	case "tab", "right":
		m.colIdx = (m.colIdx + 1) % len(columns)
		m.taskIdx = 0

	case "shift+tab", "left":
		m.colIdx = (m.colIdx - 1 + len(columns)) % len(columns)
		m.taskIdx = 0

	case "up":
		if m.taskIdx > 0 {
			m.taskIdx--
		}

	case "down":
		tasks := m.currentTasks()
		if m.taskIdx < len(tasks)-1 {
			m.taskIdx++
		}

	case "r":
		return m, m.reloadCmd()

	case "/", "f":
		m.mode = modeFilterInput

	case "F":
		m.filterProject = ""
		m.filterText = ""
		m.taskIdx = 0
		m.status = &statusMsg{"Filtros removidos", true}

	case "p":
		m.projectList = nil
		for _, p := range m.data.Projects {
			m.projectList = append(m.projectList, p.Name)
		}
		m.projectCursor = 0
		m.mode = modeProjectSelect

	case "i":
		return m.startMove(parser.StatusInProgress)
	case "t":
		return m.startMove(parser.StatusTodo)
	case "d":
		return m.startMove(parser.StatusDone)
	case "x":
		return m.startMove(parser.StatusCancelled)

	case "c":
		if m.selectedTask() != nil {
			m.commentDraft = ""
			m.mode = modeComment
		}
	}
	return m, nil
}

func (m boardModel) startMove(newStatus parser.TaskStatus) (tea.Model, tea.Cmd) {
	task := m.selectedTask()
	if task == nil || task.Status == newStatus {
		return m, nil
	}
	m.confirmTarget = newStatus
	m.mode = modeConfirm
	return m, nil
}

func (m boardModel) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "s", "y", "enter":
		task := m.selectedTask()
		if task == nil {
			m.mode = modeNormal
			return m, nil
		}
		target := m.confirmTarget
		m.mode = modeNormal
		m.status = &statusMsg{"Movendo tarefa…", true}
		return m, m.moveCmd(task, target)
	default:
		m.mode = modeNormal
	}
	return m, nil
}

func (m boardModel) handleComment(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if strings.TrimSpace(m.commentDraft) == "" {
			m.mode = modeNormal
			return m, nil
		}
		task := m.selectedTask()
		if task == nil {
			m.mode = modeNormal
			return m, nil
		}
		draft := m.commentDraft
		m.commentDraft = ""
		m.mode = modeNormal
		return m, m.commentCmd(task, draft)
	case "esc", "ctrl+c":
		m.commentDraft = ""
		m.mode = modeNormal
	case "backspace":
		if len(m.commentDraft) > 0 {
			runes := []rune(m.commentDraft)
			m.commentDraft = string(runes[:len(runes)-1])
		}
	default:
		if len(msg.Runes) > 0 {
			m.commentDraft += string(msg.Runes)
		}
	}
	return m, nil
}

func (m boardModel) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = modeNormal
		m.taskIdx = 0
	case "esc", "ctrl+c":
		m.filterText = ""
		m.mode = modeNormal
		m.taskIdx = 0
	case "backspace":
		if len(m.filterText) > 0 {
			runes := []rune(m.filterText)
			m.filterText = string(runes[:len(runes)-1])
		}
		m.taskIdx = 0
	default:
		if len(msg.Runes) > 0 {
			m.filterText += string(msg.Runes)
			m.taskIdx = 0
		}
	}
	return m, nil
}

func (m boardModel) handleProjectSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.projectCursor > 0 {
			m.projectCursor--
		}
	case "down":
		if m.projectCursor < len(m.projectList) {
			m.projectCursor++
		}
	case "enter":
		if m.projectCursor == 0 {
			m.filterProject = ""
		} else {
			m.filterProject = m.projectList[m.projectCursor-1]
		}
		m.mode = modeNormal
		m.taskIdx = 0
	case "esc", "ctrl+c", "q":
		m.mode = modeNormal
	}
	return m, nil
}

// ── View ────────────────────────────────────────────────────────────────────

func (m boardModel) View() string {
	w := m.width
	if w == 0 {
		w = 80
	}

	switch m.mode {
	case modeProjectSelect:
		return m.viewProjectSelect(w)
	case modeConfirm:
		return m.viewConfirm(w)
	}

	var sb strings.Builder
	hr := styleDim.Render(strings.Repeat("─", w))
	dothr := styleDim.Render(strings.Repeat("╌", w))

	// Header
	sb.WriteString(hr + "\n")
	totalLabel := fmt.Sprintf(" %d tarefas ", len(m.data.AllTasks))
	if m.hasFilter() {
		count := 0
		for _, col := range columns {
			count += len(m.filteredTasks(col.id))
		}
		totalLabel = styleYellow.Render(fmt.Sprintf(" %d/%d ", count, len(m.data.AllTasks)))
	} else {
		totalLabel = styleDim.Render(totalLabel)
	}
	sb.WriteString(styleTitle.Render(fmt.Sprintf(" 🗂  %s ", m.boardTitle)) + totalLabel + "\n")

	// Filter bar
	if m.hasFilter() || m.mode == modeFilterInput {
		sb.WriteString(styleDim.Render(strings.Repeat("┄", w)) + "\n")
		var parts []string
		if m.filterProject != "" {
			parts = append(parts, styleYellow.Render("  projeto: "+m.filterProject))
		}
		if m.mode == modeFilterInput {
			parts = append(parts, styleYellow.Render("  busca: "+m.filterText)+styleCyan.Render("▌"))
		} else if m.filterText != "" {
			parts = append(parts, styleYellow.Render(fmt.Sprintf(`  busca: "%s"`, m.filterText)))
		}
		sb.WriteString(styleDim.Render(" 🔍 ") + strings.Join(parts, styleDim.Render("  ·  ")) + "\n")
	}

	sb.WriteString(hr + "\n")

	// Kanban lanes
	sb.WriteString(m.viewLanes(w) + "\n")

	// Detail panel
	dothrLine := dothr + "\n"
	if sel := m.selectedTask(); sel != nil {
		sb.WriteString(dothrLine)
		sb.WriteString(styleDim.Render("  origem     : ") + fmt.Sprintf("%s:%d", sel.SourceFile, sel.LineNumber) + "\n")
		if sel.Type != "" {
			sb.WriteString(styleDim.Render("  tipo       : ") + sel.Type + "\n")
		}
		if sel.Refs != "" {
			sb.WriteString(styleDim.Render("  refs       : ") + styleCyan.Render(sel.Refs) + "\n")
		}
		if len(sel.Comments) > 0 {
			sb.WriteString(styleDim.Render(fmt.Sprintf("  comentários (%d):\n", len(sel.Comments))))
			cmts := sel.Comments
			if len(cmts) > 5 {
				omitted := len(cmts) - 5
				cmts = cmts[omitted:]
				sb.WriteString(styleDim.Render(fmt.Sprintf("    … %d anterior(es) omitido(s)\n", omitted)))
			}
			for _, c := range cmts {
				cparts := strings.SplitN(c.Text, " — ", 2)
				ts := cparts[0]
				body := ""
				if len(cparts) > 1 {
					body = cparts[1]
				}
				sb.WriteString(styleDim.Render("    · ") + styleDim.Render(ts))
				if body != "" {
					sb.WriteString(styleDim.Render(" — ") + body)
				}
				sb.WriteString("\n")
			}
		}
	}

	// Comment input
	if m.mode == modeComment {
		sb.WriteString(dothrLine)
		sb.WriteString(styleYellow.Render("  💬 Comentário: ") + m.commentDraft + styleCyan.Render("▌") + "\n")
		sb.WriteString(styleDim.Render("  Enter: confirmar  Esc: cancelar") + "\n")
	}

	// Status message
	if m.status != nil {
		sb.WriteString(dothrLine)
		if m.status.ok {
			sb.WriteString(styleGreen.Render("  ✓ "+m.status.text) + "\n")
		} else {
			sb.WriteString(styleRed.Render("  ✗ "+m.status.text) + "\n")
		}
	}

	sb.WriteString(hr + "\n")
	// Help line
	if m.mode == modeFilterInput {
		sb.WriteString(styleDim.Render("  Digite para filtrar  Enter: confirmar  Esc: cancelar") + "\n")
	} else {
		sb.WriteString(
			styleDim.Render("  ←→: lane") +
				styleDim.Render("  ↑↓: item") +
				styleDim.Render("  i/t/d/x: mover") +
				styleDim.Render("  c: comentar") +
				styleDim.Render("  /: busca") +
				styleDim.Render("  p: projeto") +
				styleDim.Render("  F: limpar") +
				styleDim.Render("  r: reload") +
				styleDim.Render("  q: sair") + "\n",
		)
	}
	sb.WriteString(hr + "\n")

	return sb.String()
}

// viewLanes renders all four kanban columns side-by-side.
func (m boardModel) viewLanes(totalWidth int) string {
	const sep = "│"
	numCols := len(columns)
	// each lane gets equal share; separator costs 1 char each (numCols-1 separators)
	laneW := (totalWidth - (numCols - 1)) / numCols
	if laneW < 10 {
		laneW = 10
	}

	laneStrs := make([]string, numCols)
	for i, col := range columns {
		laneStrs[i] = m.viewLane(col, i, laneW)
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		interleave(laneStrs, styleDim.Render(sep))...,
	)
}

// interleave inserts sep between each element of strs.
func interleave(strs []string, sep string) []string {
	if len(strs) == 0 {
		return nil
	}
	out := make([]string, 0, len(strs)*2-1)
	for i, s := range strs {
		out = append(out, s)
		if i < len(strs)-1 {
			out = append(out, sep)
		}
	}
	return out
}

// viewLane renders a single kanban lane.
func (m boardModel) viewLane(col column, colIndex int, w int) string {
	isActive := colIndex == m.colIdx
	tasks := m.filteredTasks(col.id)
	total := len(m.data.ByStatus[col.id])

	// Header
	countStr := fmt.Sprintf("%d", len(tasks))
	if m.hasFilter() {
		countStr = fmt.Sprintf("%d/%d", len(tasks), total)
	}
	headerText := fmt.Sprintf("%s %s (%s)", col.icon, col.label, countStr)
	var header string
	if isActive {
		header = styleBold.Render(styleCyan.Render(headerText))
	} else {
		header = styleDim.Render(headerText)
	}
	// pad/trim header to lane width
	hW := lipgloss.Width(header)
	if hW < w {
		header += strings.Repeat(" ", w-hW)
	}

	var laneLines []string
	laneLines = append(laneLines, header)

	// divider under header
	if isActive {
		laneLines = append(laneLines, styleCyan.Render(strings.Repeat("─", w)))
	} else {
		laneLines = append(laneLines, styleDim.Render(strings.Repeat("─", w)))
	}

	// Task rows
	if len(tasks) == 0 {
		empty := styleDim.Render("  (vazio)")
		eW := lipgloss.Width(empty)
		if eW < w {
			empty += strings.Repeat(" ", w-eW)
		}
		laneLines = append(laneLines, empty)
	} else {
		start := 0
		if isActive {
			start = m.taskIdx - maxVisible + 1
			if start < 0 {
				start = 0
			}
		}
		end := start + maxVisible
		if end > len(tasks) {
			end = len(tasks)
		}
		for i := start; i < end; i++ {
			t := tasks[i]
			isSel := isActive && i == m.taskIdx
			cursor := " "
			if isSel {
				cursor = styleCyan.Render("▶")
			}
			icon := taskIcon(t.Status, isSel)
			tag := ""
			if t.Type != "" {
				tag = styleDim.Render(fmt.Sprintf("[%s]", t.Type))
			}
			cmtBadge := ""
			if len(t.Comments) > 0 {
				cmtBadge = styleDim.Render(fmt.Sprintf("💬%d", len(t.Comments)))
			}
			// suffix: tag + cmtBadge (plain width, no color codes for measuring)
			suffix := ""
			if tag != "" {
				suffix += " " + tag
			}
			if cmtBadge != "" {
				suffix += " " + cmtBadge
			}
			// available width for text: w - cursor(1) - space(1) - icon(1) - space(1) - suffix
			suffixW := lipgloss.Width(suffix)
			textBudget := w - 4 - suffixW
			if textBudget < 1 {
				textBudget = 1
			}
			text := t.Text
			textRunes := []rune(text)
			if len(textRunes) > textBudget {
				text = string(textRunes[:textBudget-1]) + "…"
			} else {
				// pad to fill lane width
				text += strings.Repeat(" ", textBudget-len([]rune(text)))
			}
			if isSel {
				text = styleSelected.Render(text)
			}
			line := fmt.Sprintf("%s %s %s%s", cursor, icon, text, suffix)
			laneLines = append(laneLines, line)
		}
		if len(tasks) > maxVisible {
			more := styleDim.Render(fmt.Sprintf("  … %d mais", len(tasks)-maxVisible))
			mW := lipgloss.Width(more)
			if mW < w {
				more += strings.Repeat(" ", w-mW)
			}
			laneLines = append(laneLines, more)
		}
	}

	return strings.Join(laneLines, "\n")
}

func (m boardModel) viewConfirm(w int) string {
	col := columns[0]
	for _, c := range columns {
		if c.id == m.confirmTarget {
			col = c
			break
		}
	}
	task := m.selectedTask()
	if task == nil {
		return ""
	}
	hr := styleDim.Render(strings.Repeat("─", w))
	text := task.Text
	if len([]rune(text)) > 40 {
		text = string([]rune(text)[:39]) + "…"
	}
	return hr + "\n" +
		styleYellow.Render(fmt.Sprintf(`  Mover "%s" → %s?`, text, col.label)) + "\n" +
		styleDim.Render("  [s] sim  [n] não") + "\n" +
		hr + "\n"
}

func (m boardModel) viewProjectSelect(w int) string {
	hr := styleDim.Render(strings.Repeat("─", w))
	var sb strings.Builder
	sb.WriteString(hr + "\n")
	sb.WriteString(styleBold.Render(styleCyan.Render(" Filtrar por projeto\n\n")))
	cursor := " "
	if m.projectCursor == 0 {
		cursor = styleCyan.Render("▶")
	}
	sb.WriteString(fmt.Sprintf(" %s 0) — Todos os projetos —\n", cursor))
	for i, name := range m.projectList {
		cursor = " "
		if m.projectCursor == i+1 {
			cursor = styleCyan.Render("▶")
		}
		sb.WriteString(fmt.Sprintf(" %s %d) %s\n", cursor, i+1, name))
	}
	sb.WriteString("\n" + styleDim.Render("  ↑↓: navegar  Enter: selecionar  Esc: cancelar") + "\n")
	sb.WriteString(hr + "\n")
	return sb.String()
}

func taskIcon(status parser.TaskStatus, selected bool) string {
	switch status {
	case parser.StatusTodo:
		if selected {
			return styleCyan.Render("○")
		}
		return styleDim.Render("○")
	case parser.StatusInProgress:
		return styleYellow.Render("◐")
	case parser.StatusDone:
		return styleGreen.Render("●")
	case parser.StatusCancelled:
		return styleRed.Render("✕")
	}
	return "?"
}

// ── RunBoard ──────────────────────────────────────────────────────────────────

// RunBoard is the entry point for `otb board`.
func RunBoard(flags BoardFlags) error {
	vaultPath, err := vault.Resolve(getCwd(), flags.VaultPath)
	if err != nil {
		return err
	}

	data, err := reader.Read(vaultPath, nil)
	if err != nil {
		return fmt.Errorf("reading vault: %w", err)
	}

	if len(data.AllTasks) == 0 {
		fmt.Println("Nenhuma tarefa encontrada no vault.")
		return nil
	}

	// Find first column with tasks
	startCol := 0
	for i, col := range columns {
		if len(data.ByStatus[col.id]) > 0 {
			startCol = i
			break
		}
	}

	// Apply --project filter
	filterProject := ""
	if flags.Project != "" {
		found := false
		for _, p := range data.Projects {
			if strings.Contains(strings.ToLower(p.Name), strings.ToLower(flags.Project)) {
				filterProject = p.Name
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("projeto não encontrado: %q", flags.Project)
		}
	}

	m := boardModel{
		data:          data,
		vaultPath:     vaultPath,
		boardTitle:    vaultBaseName(vaultPath),
		colIdx:        startCol,
		filterProject: filterProject,
		width:         80,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

func vaultBaseName(path string) string {
	parts := strings.Split(filepath.Clean(path), string(os.PathSeparator))
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}
