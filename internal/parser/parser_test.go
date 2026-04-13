package parser_test

import (
	"strings"
	"testing"

	"github.com/pot-labs/otb/internal/parser"
)

// ── ParseTaskLine ─────────────────────────────────────────────────────────────

func TestParseTaskLine_AllStatuses(t *testing.T) {
	cases := []struct {
		line string
		want parser.TaskStatus
	}{
		{"- [ ] todo task", parser.StatusTodo},
		{"- [/] in progress task", parser.StatusInProgress},
		{"- [x] done task", parser.StatusDone},
		{"- [X] done uppercase", parser.StatusDone},
		{"- [-] cancelled task", parser.StatusCancelled},
	}
	for _, c := range cases {
		task := parser.ParseTaskLine(c.line, "test.md", 1)
		if task == nil {
			t.Fatalf("ParseTaskLine(%q) = nil; expected task", c.line)
		}
		if task.Status != c.want {
			t.Errorf("status: got %q want %q for %q", task.Status, c.want, c.line)
		}
	}
}

func TestParseTaskLine_InlineFields(t *testing.T) {
	line := "- [ ] My task [type:: technical] [refs:: PR#42, commit:abc]"
	task := parser.ParseTaskLine(line, "f.md", 5)
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if task.Type != "technical" {
		t.Errorf("type: got %q want %q", task.Type, "technical")
	}
	if task.Refs != "PR#42, commit:abc" {
		t.Errorf("refs: got %q want %q", task.Refs, "PR#42, commit:abc")
	}
	// Inline fields must be stripped from display text
	if strings.Contains(task.Text, "type::") || strings.Contains(task.Text, "refs::") {
		t.Errorf("inline fields leaked into display text: %q", task.Text)
	}
}

func TestParseTaskLine_Fingerprint(t *testing.T) {
	task := parser.ParseTaskLine("- [ ] hello world", "proj.md", 7)
	if task == nil {
		t.Fatal("nil task")
	}
	if !strings.HasPrefix(task.Fingerprint, "proj.md:L7:") {
		t.Errorf("fingerprint format wrong: %q", task.Fingerprint)
	}
}

func TestParseTaskLine_NonTaskLine(t *testing.T) {
	cases := []string{
		"plain text",
		"# heading",
		"",
		"  - no checkbox here",
	}
	for _, line := range cases {
		if task := parser.ParseTaskLine(line, "f.md", 1); task != nil {
			t.Errorf("expected nil for %q, got task", line)
		}
	}
}

func TestParseTaskLine_RejectMultiCharCheckbox(t *testing.T) {
	// Multi-rune checkbox should be rejected
	cases := []string{
		"- [xx] bad",
		"- [/ ] bad",
		"- [ /] bad",
	}
	for _, line := range cases {
		if task := parser.ParseTaskLine(line, "f.md", 1); task != nil {
			t.Errorf("expected nil for multi-char checkbox %q, got task", line)
		}
	}
}

// ── Security tests ─────────────────────────────────────────────────────────

func TestParseTaskLine_ANSIInjection(t *testing.T) {
	line := "- [ ] \x1b[31mred task\x1b[0m [type:: \x1b[32minjected\x1b[0m]"
	task := parser.ParseTaskLine(line, "f.md", 1)
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if strings.Contains(task.Text, "\x1b") {
		t.Errorf("ANSI not stripped from text: %q", task.Text)
	}
	if strings.Contains(task.Type, "\x1b") {
		t.Errorf("ANSI not stripped from type: %q", task.Type)
	}
}

func TestParseTaskLine_BidiInjection(t *testing.T) {
	// U+202E RLO — text spoofing
	line := "- [ ] safe\u202Eevil task"
	task := parser.ParseTaskLine(line, "f.md", 1)
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if strings.ContainsRune(task.Text, 0x202E) {
		t.Errorf("bidi RLO not stripped: %q", task.Text)
	}
}

func TestParseTaskLine_NullByteInjection(t *testing.T) {
	line := "- [ ] task\x00with null"
	task := parser.ParseTaskLine(line, "f.md", 1)
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if strings.ContainsRune(task.Text, 0) {
		t.Errorf("null byte not stripped: %q", task.Text)
	}
}

func TestParseTaskLine_OSCInjection(t *testing.T) {
	line := "- [ ] task\x1b]0;pwned\x07after"
	task := parser.ParseTaskLine(line, "f.md", 1)
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if strings.Contains(task.Text, "\x1b") {
		t.Errorf("OSC injection not stripped: %q", task.Text)
	}
}

// ── ParseProjectFile ──────────────────────────────────────────────────────────

func TestParseProjectFile_CommentsAttachedToParent(t *testing.T) {
	content := strings.Join([]string{
		"- [ ] first task",
		"  - comment:: 2026-04-12 10:00 @alice — note",
		"  - comment:: 2026-04-12 11:00 @bob — follow-up",
		"",
		"- [/] second task",
		"  - comment:: 2026-04-12 12:00 @carol — in review",
	}, "\n")

	tasks := parser.ParseProjectFile(content, "proj.md")
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if len(tasks[0].Comments) != 2 {
		t.Errorf("first task: expected 2 comments, got %d", len(tasks[0].Comments))
	}
	if len(tasks[1].Comments) != 1 {
		t.Errorf("second task: expected 1 comment, got %d", len(tasks[1].Comments))
	}
}

func TestParseProjectFile_CommentsNotLeakedAcrossTasks(t *testing.T) {
	content := strings.Join([]string{
		"- [ ] task one",
		"- [ ] task two",
		"  - comment:: 2026-04-12 10:00 @user — only for task two",
	}, "\n")

	tasks := parser.ParseProjectFile(content, "f.md")
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if len(tasks[0].Comments) != 0 {
		t.Errorf("task one should have no comments, got %d", len(tasks[0].Comments))
	}
	if len(tasks[1].Comments) != 1 {
		t.Errorf("task two should have 1 comment, got %d", len(tasks[1].Comments))
	}
}
