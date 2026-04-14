// Package parser parses Obsidian Tasks plugin task lines from Markdown files.
//
// Supported syntax:
//
//	- [ ]  Todo
//	- [/]  In Progress
//	- [x]  Done  (also [X])
//	- [-]  Cancelled
//
// Inline fields:
//
//	[type:: value]
//	[refs:: value]
//
// Comment subtopics (indented, attached to parent task):
//
//	  - comment:: 2026-04-12 14:30 @user — note text
package parser

import (
	"regexp"
	"strings"

	"github.com/pot-labs/otb/internal/sanitize"
)

// TaskStatus represents the four possible Obsidian Tasks plugin states.
type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
	StatusCancelled  TaskStatus = "cancelled"
	StatusBacklog    TaskStatus = "backlog"
)

// TaskComment is a single comment line attached to a task.
type TaskComment struct {
	Text       string
	LineNumber int
}

// Task is a parsed task with all metadata.
type Task struct {
	Text        string       // display text (inline fields stripped, sanitized)
	Status      TaskStatus
	Type        string       // optional [type:: ...] value
	Refs        string       // optional [refs:: ...] value
	Comments    []TaskComment
	SourceFile  string
	LineNumber  int
	Fingerprint string       // "<sourceFile>:L<line>:<normalizedText>"
}

// Field limits (in runes) — any value beyond these is truncated.
const (
	maxText    = 300
	maxType    = 100
	maxRefs    = 200
	maxComment = 200
)

var (
	reTask    = regexp.MustCompile(`^(\s*)[-*]\s+\[([^\]]*)\]\s+(.+)$`)
	reField   = regexp.MustCompile(`\[[^\]]+::\s*[^\]]+\]`)
	reType    = regexp.MustCompile(`\[type::\s*([^\]]+)\]`)
	reRefs    = regexp.MustCompile(`\[refs::\s*([^\]]+)\]`)
	reComment = regexp.MustCompile(`^(\s{2,})[-*]\s+comment::\s+(.+)$`)
)

func toStatus(ch string) TaskStatus {
	switch ch {
	case "/":
		return StatusInProgress
	case "x", "X":
		return StatusDone
	case "-":
		return StatusCancelled
	case "b":
		return StatusBacklog
	default:
		return StatusTodo
	}
}

func extractField(raw, field string) string {
	var re *regexp.Regexp
	switch field {
	case "type":
		re = reType
	case "refs":
		re = reRefs
	default:
		return ""
	}
	m := re.FindStringSubmatch(raw)
	if m == nil {
		return ""
	}
	limit := maxType
	if field == "refs" {
		limit = maxRefs
	}
	return sanitize.ForDisplay(strings.TrimSpace(m[1]), limit)
}

func stripFields(s string) string {
	return strings.Join(strings.Fields(reField.ReplaceAllString(s, "")), " ")
}

func fingerprint(sourceFile string, lineNumber int, normalizedText string) string {
	return sourceFile + ":L" + itoa(lineNumber) + ":" + normalizedText
}

// itoa is a minimal int-to-string helper to avoid importing fmt.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

// ParseTaskLine parses a single Markdown line into a Task.
// Returns nil if the line is not a task line.
func ParseTaskLine(line, sourceFile string, lineNumber int) *Task {
	m := reTask.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	checkChar := m[2]
	// Security: reject multi-rune checkbox values
	if len([]rune(checkChar)) > 1 {
		return nil
	}
	rawContent := m[3]
	sanitizedRaw := sanitize.ForDisplay(rawContent, maxText)
	displayText := sanitize.ForDisplay(stripFields(sanitizedRaw), maxText)

	return &Task{
		Text:        displayText,
		Status:      toStatus(checkChar),
		Type:        extractField(sanitizedRaw, "type"),
		Refs:        extractField(sanitizedRaw, "refs"),
		Comments:    nil,
		SourceFile:  sourceFile,
		LineNumber:  lineNumber,
		Fingerprint: fingerprint(sourceFile, lineNumber, displayText),
	}
}

func parseCommentLine(line string, lineNumber int) *TaskComment {
	m := reComment.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	text := sanitize.ForDisplay(strings.TrimSpace(m[2]), maxComment)
	if text == "" {
		return nil
	}
	return &TaskComment{Text: text, LineNumber: lineNumber}
}

// ParseProjectFile parses all task lines (with attached comments) from
// the content of a single Markdown file.
func ParseProjectFile(content, sourceFile string) []Task {
	var tasks []Task
	lines := strings.Split(content, "\n")
	var lastTask *Task

	for i, line := range lines {
		lineNumber := i + 1

		if lastTask != nil {
			if cmt := parseCommentLine(line, lineNumber); cmt != nil {
				lastTask.Comments = append(lastTask.Comments, *cmt)
				continue
			}
		}

		if task := ParseTaskLine(line, sourceFile, lineNumber); task != nil {
			tasks = append(tasks, *task)
			lastTask = &tasks[len(tasks)-1]
			continue
		}

		if strings.TrimSpace(line) != "" {
			lastTask = nil
		}
	}

	return tasks
}
