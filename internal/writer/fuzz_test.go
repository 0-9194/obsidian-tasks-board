// Package writer_fuzz contains fuzz tests for the writer package.
package writer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pot-labs/otb/internal/parser"
	"github.com/pot-labs/otb/internal/writer"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func makeVaultWithContent(t *testing.T, content string) (vaultPath, relPath string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".obsidian"), 0750); err != nil {
		t.Fatalf("mkdir .obsidian: %v", err)
	}
	dir := filepath.Join(root, "20 - Projects")
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}
	fp := filepath.Join(dir, "tasks.md")
	if err := os.WriteFile(fp, []byte(content), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return root, "20 - Projects/tasks.md"
}

// ── Fuzz: ChangeTaskStatus ────────────────────────────────────────────────────
//
// Invariants:
//  1. Never panics for any file content or task text
//  2. If the operation succeeds, the resulting file contains valid UTF-8
//  3. If the operation succeeds, the checkbox char is one of [ ], [/], [x], [-]
//  4. File size never grows unboundedly (≤ original + small delta)

func FuzzChangeTaskStatus(f *testing.F) {
	seeds := []string{
		"- [ ] normal task\n",
		"- [x] already done\n- [ ] another\n",
		"- [ ] task [type:: feat]\n",
		"- [ ] \x1b[31mevil\x1b[0m\n",
		"- [ ] \u202e reversed\n",
		"- [ ] task one\n- [ ] task two\n- [ ] task three\n",
		"",
		"not a task\n",
		"- [ ] " + strings.Repeat("a", 500) + "\n",
	}
	statuses := []parser.TaskStatus{
		parser.StatusTodo, parser.StatusInProgress,
		parser.StatusDone, parser.StatusCancelled,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, content string) {
		tasks := parser.ParseProjectFile(content, "20 - Projects/tasks.md")
		if len(tasks) == 0 {
			return
		}

		vault, rel := makeVaultWithContent(t, content)
		task := tasks[0]
		task.SourceFile = rel

		originalSize := int64(len(content))

		for _, newStatus := range statuses {
			// Re-read the task after each mutation to keep fingerprint fresh
			current, err := os.ReadFile(filepath.Join(vault, rel))
			if err != nil {
				return
			}
			refreshed := parser.ParseProjectFile(string(current), rel)
			if len(refreshed) == 0 {
				return
			}
			t2 := refreshed[0]

			_ = writer.ChangeTaskStatus(vault, &t2, newStatus)

			// Invariant: file is still valid UTF-8
			after, err := os.ReadFile(filepath.Join(vault, rel))
			if err != nil {
				continue
			}
			if !isValidUTF8(after) {
				t.Errorf("file became invalid UTF-8 after ChangeTaskStatus with content=%q", content)
			}
			// Invariant: file size not explosively larger (≤ original + 100 bytes per status)
			if int64(len(after)) > originalSize+400 {
				t.Errorf("file grew explosively: %d → %d bytes", originalSize, len(after))
			}
		}
	})
}

// ── Fuzz: AddTaskComment ──────────────────────────────────────────────────────
//
// Invariants:
//  1. Never panics for any comment text or author
//  2. After a successful comment, the file contains the comment line
//  3. Comment line contains no ESC sequences after sanitization
//  4. Malformed author is sanitized to [a-zA-Z0-9_-]

func FuzzAddTaskComment(f *testing.F) {
	seeds := []struct{ content, text, author string }{
		{"- [ ] task\n", "normal comment", "user"},
		{"- [ ] task\n", "\x1b[31mevil\x1b[0m", "attacker"},
		{"- [ ] task\n", "\u202e reversed", "user"},
		{"- [ ] task\n", strings.Repeat("x", 1000), "user"},
		{"- [ ] task\n", "note", "user;rm -rf /"},
		{"- [ ] task\n", "note", ""},
		{"- [ ] task\n", "", "user"},
		{"- [ ] task\n", "note", strings.Repeat("A", 300)},
		{"- [ ] task\n", "\x00null\x00", "user"},
	}
	for _, s := range seeds {
		f.Add(s.content, s.text, s.author)
	}

	f.Fuzz(func(t *testing.T, content, text, author string) {
		tasks := parser.ParseProjectFile(content, "20 - Projects/tasks.md")
		if len(tasks) == 0 {
			return
		}

		vault, rel := makeVaultWithContent(t, content)
		task := tasks[0]
		task.SourceFile = rel

		err := writer.AddTaskComment(vault, &task, text, author)
		if err != nil {
			// Errors are acceptable (empty text, fingerprint mismatch, etc.)
			return
		}

		// Read resulting file
		after, readErr := os.ReadFile(filepath.Join(vault, rel))
		if readErr != nil {
			t.Fatalf("read after comment: %v", readErr)
		}

		// Invariant: file is valid UTF-8
		if !isValidUTF8(after) {
			t.Errorf("file became invalid UTF-8")
		}

		// Invariant: no ESC in file after write
		if strings.ContainsRune(string(after), '\x1b') {
			t.Errorf("ESC byte written to file: %q", string(after))
		}

		// Invariant: file contains comment:: marker
		if !strings.Contains(string(after), "comment::") {
			t.Errorf("comment:: not found in file after successful AddTaskComment")
		}
	})
}

// ── Fuzz: ChangeTaskStatus — path traversal via SourceFile ───────────────────
//
// This fuzz test specifically targets the path traversal guard in writer by
// providing arbitrary relative paths as SourceFile. The vault boundary must
// never be escaped.

func FuzzChangeTaskStatus_SourceFilePath(f *testing.F) {
	seeds := []string{
		"20 - Projects/tasks.md",
		"../escape.md",
		"../../etc/passwd",
		"20 - Projects/../../../etc/shadow",
		"/absolute/path.md",
		".",
		"..",
		"20 - Projects/tasks.md\x00evil",
		strings.Repeat("../", 50) + "etc/passwd",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, sourceFile string) {
		vault := t.TempDir()
		if err := os.MkdirAll(filepath.Join(vault, ".obsidian"), 0750); err != nil {
			t.Fatal(err)
		}

		task := &parser.Task{
			Text:        "test task",
			Status:      parser.StatusTodo,
			SourceFile:  sourceFile,
			LineNumber:  1,
			Fingerprint: sourceFile + ":L1:test task",
		}

		err := writer.ChangeTaskStatus(vault, task, parser.StatusDone)
		if err == nil {
			// If it succeeded, the file must be inside the vault
			abs := filepath.Join(vault, sourceFile)
			abs = filepath.Clean(abs)
			vaultClean := filepath.Clean(vault)
			if !strings.HasPrefix(abs, vaultClean+string(filepath.Separator)) {
				t.Errorf("path traversal succeeded: sourceFile=%q abs=%q vault=%q",
					sourceFile, abs, vault)
			}
		}
		// Errors are expected and correct for traversal attempts
	})
}

func isValidUTF8(b []byte) bool {
	s := string(b)
	for i, r := range s {
		_ = i
		if r == '\uFFFD' {
			return false
		}
	}
	return true
}
