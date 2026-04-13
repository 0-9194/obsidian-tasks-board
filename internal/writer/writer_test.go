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

func setupVaultWithFile(t *testing.T, content string) (vaultPath, filePath string) {
	t.Helper()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".obsidian"), 0750); err != nil {
		t.Fatal(err)
	}
	projDir := filepath.Join(tmp, "20 - Projects")
	if err := os.MkdirAll(projDir, 0750); err != nil {
		t.Fatal(err)
	}
	fp := filepath.Join(projDir, "tasks.md")
	if err := os.WriteFile(fp, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return tmp, fp
}

func parseFirst(t *testing.T, content, relPath string) *parser.Task {
	t.Helper()
	tasks := parser.ParseProjectFile(content, relPath)
	if len(tasks) == 0 {
		t.Fatal("no tasks found in content")
	}
	task := tasks[0]
	return &task
}

// ── ChangeTaskStatus ──────────────────────────────────────────────────────────

func TestChangeTaskStatus_HappyPath(t *testing.T) {
	content := "- [ ] my task\n"
	vault, fp := setupVaultWithFile(t, content)

	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	if err := writer.ChangeTaskStatus(vault, task, parser.StatusDone); err != nil {
		t.Fatalf("ChangeTaskStatus error: %v", err)
	}

	updated, _ := os.ReadFile(fp)
	if !strings.Contains(string(updated), "- [x]") {
		t.Errorf("expected [x] checkbox, got:\n%s", string(updated))
	}
}

func TestChangeTaskStatus_InProgress(t *testing.T) {
	content := "- [ ] start task\n"
	vault, fp := setupVaultWithFile(t, content)
	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	if err := writer.ChangeTaskStatus(vault, task, parser.StatusInProgress); err != nil {
		t.Fatalf("ChangeTaskStatus error: %v", err)
	}

	data, _ := os.ReadFile(fp)
	if !strings.Contains(string(data), "- [/]") {
		t.Errorf("expected [/] checkbox, got:\n%s", string(data))
	}
}

func TestChangeTaskStatus_NoopWhenSameStatus(t *testing.T) {
	content := "- [ ] already todo\n"
	vault, _ := setupVaultWithFile(t, content)
	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	// Should not error, no write needed
	if err := writer.ChangeTaskStatus(vault, task, parser.StatusTodo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChangeTaskStatus_FingerprintMismatch(t *testing.T) {
	content := "- [ ] original task\n"
	vault, fp := setupVaultWithFile(t, content)
	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	// Modify file after reading
	if err := os.WriteFile(fp, []byte("- [ ] completely different\n"), 0600); err != nil {
		t.Fatal(err)
	}

	err := writer.ChangeTaskStatus(vault, task, parser.StatusDone)
	if err == nil {
		t.Fatal("expected fingerprint mismatch error, got nil")
	}
}

func TestChangeTaskStatus_AtomicWriteTempFileCleanup(t *testing.T) {
	content := "- [ ] atomic task\n"
	vault, _ := setupVaultWithFile(t, content)
	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	if err := writer.ChangeTaskStatus(vault, task, parser.StatusDone); err != nil {
		t.Fatalf("ChangeTaskStatus error: %v", err)
	}

	// Verify no .tmp-otb-* files left behind
	dir := filepath.Join(vault, "20 - Projects")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-otb-") {
			t.Errorf("temp file not cleaned up: %s", e.Name())
		}
	}
}

// ── AddTaskComment ─────────────────────────────────────────────────────────────

func TestAddTaskComment_AppendedCorrectly(t *testing.T) {
	content := "- [ ] task with comment\n"
	vault, fp := setupVaultWithFile(t, content)
	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	if err := writer.AddTaskComment(vault, task, "test comment", "alice"); err != nil {
		t.Fatalf("AddTaskComment error: %v", err)
	}

	data, _ := os.ReadFile(fp)
	if !strings.Contains(string(data), "comment::") {
		t.Errorf("comment not found in file:\n%s", string(data))
	}
	if !strings.Contains(string(data), "@alice") {
		t.Errorf("author not found in file:\n%s", string(data))
	}
	if !strings.Contains(string(data), "test comment") {
		t.Errorf("comment text not found in file:\n%s", string(data))
	}
}

func TestAddTaskComment_EmptyTextRejected(t *testing.T) {
	content := "- [ ] task\n"
	vault, _ := setupVaultWithFile(t, content)
	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	err := writer.AddTaskComment(vault, task, "", "user")
	if err == nil {
		t.Fatal("expected error for empty comment text, got nil")
	}
}

// ── Security tests ─────────────────────────────────────────────────────────

func TestChangeTaskStatus_PathTraversal(t *testing.T) {
	content := "- [ ] task\n"
	vault, _ := setupVaultWithFile(t, content)
	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	// Inject traversal into SourceFile
	task.SourceFile = "../../etc/passwd"

	err := writer.ChangeTaskStatus(vault, task, parser.StatusDone)
	if err == nil {
		t.Fatal("expected path traversal error, got nil")
	}
}

func TestAddTaskComment_ANSIInjection(t *testing.T) {
	content := "- [ ] task\n"
	vault, fp := setupVaultWithFile(t, content)
	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	malicious := "\x1b[31mred\x1b[0m injected comment"
	if err := writer.AddTaskComment(vault, task, malicious, "user"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(fp)
	if strings.Contains(string(data), "\x1b") {
		t.Errorf("ANSI not stripped from comment in file:\n%s", string(data))
	}
}

func TestAddTaskComment_AuthorInjection(t *testing.T) {
	content := "- [ ] task\n"
	vault, fp := setupVaultWithFile(t, content)
	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	// Author with shell metacharacters
	maliciousAuthor := "; rm -rf /"
	if err := writer.AddTaskComment(vault, task, "valid comment", maliciousAuthor); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(fp)
	// Extract comment line and verify author token only has safe chars [a-zA-Z0-9_-]
	commentLine := ""
	for _, l := range strings.Split(string(data), "\n") {
		if strings.Contains(l, "comment::") {
			commentLine = l
			break
		}
	}
	if commentLine == "" {
		t.Fatal("comment line not found in file")
	}
	// Locate @author and validate chars
	atIdx := strings.Index(commentLine, "@")
	if atIdx < 0 {
		t.Fatal("no @author found in comment line")
	}
	authorEnd := atIdx + 1
	for authorEnd < len(commentLine) && commentLine[authorEnd] != ' ' {
		authorEnd++
	}
	authorToken := commentLine[atIdx+1 : authorEnd]
	for _, ch := range authorToken {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
			t.Errorf("unsafe char %q in author token %q: %s", ch, authorToken, commentLine)
		}
	}
}

func TestAddTaskComment_BidiInjectionInText(t *testing.T) {
	content := "- [ ] task\n"
	vault, fp := setupVaultWithFile(t, content)
	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	// U+202E RLO bidi override
	malicious := "safe\u202Eevil comment"
	if err := writer.AddTaskComment(vault, task, malicious, "user"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(fp)
	if strings.ContainsRune(string(data), 0x202E) {
		t.Errorf("bidi RLO not stripped from comment")
	}
}

func TestChangeTaskStatus_AbsolutePathEscape(t *testing.T) {
	content := "- [ ] task\n"
	vault, _ := setupVaultWithFile(t, content)
	rel := "20 - Projects/tasks.md"
	task := parseFirst(t, content, rel)

	task.SourceFile = "/etc/hosts"
	err := writer.ChangeTaskStatus(vault, task, parser.StatusDone)
	if err == nil {
		t.Fatal("expected error for absolute path escape, got nil")
	}
}
