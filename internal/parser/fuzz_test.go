// Package parser_fuzz contains fuzz tests for the parser package.
package parser_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pot-labs/otb/internal/parser"
)

// ── Fuzz: ParseTaskLine ────────────────────────────────────────────────────────
//
// Invariants tested:
//  1. Never panics on any input
//  2. If a task is returned, Text must be valid UTF-8 and contain no control chars
//  3. Fingerprint is always non-empty when task != nil
//  4. Status is one of the four known values
//  5. Type and Refs contain no ESC bytes

func FuzzParseTaskLine(f *testing.F) {
	// Seed corpus: known-interesting inputs
	seeds := []string{
		"- [ ] normal task",
		"- [x] done",
		"- [/] in progress [type:: feat] [refs:: #42]",
		"- [-] cancelled",
		// Injection attempts
		"- [ ] task\x1b[31mred\x1b[0m",
		"- [ ] task\x00null",
		"- [ ] \u202e reversed text",
		"- [ ] \x1b]0;title\x07 osc inject",
		"- [ ] <script>alert(1)</script>",
		// Edge cases
		"",
		"not a task line",
		"- [] empty checkbox",
		"- [xx] multi char",
		strings.Repeat("A", 1000),
		"- [ ] " + strings.Repeat("界", 200),
		"* [ ] asterisk bullet",
		"  - [ ] indented task",
		"- [ ] [type:: " + strings.Repeat("x", 500) + "]",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, line string) {
		// Must not panic
		task := parser.ParseTaskLine(line, "fuzz.md", 1)

		if task == nil {
			return
		}

		// Invariant 1: Text is valid UTF-8
		if !utf8.ValidString(task.Text) {
			t.Errorf("Text is not valid UTF-8: %q", task.Text)
		}

		// Invariant 2: Text contains no ESC or C0/C1 control chars
		for i, r := range task.Text {
			if r == '\x1b' || (r < 0x20 && r != ' ') || (r >= 0x80 && r <= 0x9F) {
				t.Errorf("Text[%d] contains dangerous rune %U: %q", i, r, task.Text)
			}
		}

		// Invariant 3: Fingerprint non-empty
		if task.Fingerprint == "" {
			t.Error("Fingerprint is empty")
		}

		// Invariant 4: Status is valid
		switch task.Status {
		case parser.StatusTodo, parser.StatusInProgress, parser.StatusDone, parser.StatusCancelled:
			// ok
		default:
			t.Errorf("unexpected status %q", task.Status)
		}

		// Invariant 5: Type and Refs contain no ESC
		if strings.ContainsRune(task.Type, '\x1b') {
			t.Errorf("Type contains ESC: %q", task.Type)
		}
		if strings.ContainsRune(task.Refs, '\x1b') {
			t.Errorf("Refs contains ESC: %q", task.Refs)
		}

		// Invariant 6: Text length bounded (maxText = 300 runes + ellipsis)
		if len([]rune(task.Text)) > 302 {
			t.Errorf("Text too long: %d runes", len([]rune(task.Text)))
		}
	})
}

// ── Fuzz: ParseProjectFile ────────────────────────────────────────────────────
//
// Invariants:
//  1. Never panics on any file content
//  2. All returned tasks satisfy the same invariants as ParseTaskLine
//  3. Comments are never attached to a task from a different line

func FuzzParseProjectFile(f *testing.F) {
	seeds := []string{
		"- [ ] task one\n  - comment:: 2026-01-01 @user — note\n- [x] task two\n",
		"",
		"no tasks here\njust prose\n",
		"- [ ] task\x1b[31m injected\x1b[0m\n",
		"- [ ] \x00 null byte\n",
		"- [ ] a\n- [ ] b\n- [ ] c\n",
		"- [ ] [type:: " + strings.Repeat("x", 1000) + "]\n",
		strings.Repeat("- [ ] task\n", 500),
		"- [ ] task\n" + strings.Repeat("  - comment:: 2026-01-01 @u — c\n", 100),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, content string) {
		tasks := parser.ParseProjectFile(content, "fuzz.md")

		for i, task := range tasks {
			if !utf8.ValidString(task.Text) {
				t.Errorf("task[%d].Text not valid UTF-8", i)
			}
			for _, r := range task.Text {
				if r == '\x1b' {
					t.Errorf("task[%d].Text contains ESC", i)
				}
			}
			if task.LineNumber <= 0 {
				t.Errorf("task[%d].LineNumber <= 0: %d", i, task.LineNumber)
			}
			for j, c := range task.Comments {
				if !utf8.ValidString(c.Text) {
					t.Errorf("task[%d].Comments[%d].Text not valid UTF-8", i, j)
				}
				if c.LineNumber <= task.LineNumber {
					t.Errorf("task[%d].Comments[%d] at line %d <= task line %d",
						i, j, c.LineNumber, task.LineNumber)
				}
			}
		}
	})
}
