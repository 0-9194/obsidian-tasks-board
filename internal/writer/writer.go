// Package writer provides safe, atomic mutations to vault Markdown task lines.
//
// Safety model:
//  1. Path traversal check — sourceFile must resolve inside vaultPath
//  2. Re-read the file fresh before writing
//  3. Fingerprint verification — target line must still match task identity
//  4. Apply minimal change
//  5. Atomic write via temp file (in same directory) + os.Rename
package writer

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pot-labs/otb/internal/parser"
	"github.com/pot-labs/otb/internal/sanitize"
	"github.com/pot-labs/otb/internal/vault"
)

// ErrFingerprintMismatch is returned when the file has changed since the task
// was last read — the caller should reload and retry.
var ErrFingerprintMismatch = errors.New("fingerprint mismatch: file changed since last read")

var statusToChar = map[parser.TaskStatus]string{
	parser.StatusTodo:       " ",
	parser.StatusInProgress: "/",
	parser.StatusDone:       "x",
	parser.StatusCancelled:  "-",
}

var reCheckbox = regexp.MustCompile(`^(\s*[-*]\s+\[)[^\]]*(\].*)$`)
var reFieldStrip = regexp.MustCompile(`\[[^\]]+::\s*[^\]]+\]`)
var reAuthor = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// normalizeLine strips inline fields and collapses whitespace, matching the
// fingerprint logic in the parser.
func normalizeLine(s string) string {
	s = reFieldStrip.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

func readLines(filePath string) ([]string, string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("reading %q: %w", filePath, err)
	}
	raw := string(data)
	eol := "\n"
	if strings.Contains(raw, "\r\n") {
		eol = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	return lines, eol, nil
}

func writeLines(filePath string, lines []string, eol string) error {
	dir := filepath.Dir(filePath)

	// Generate temp filename in the same directory (never /tmp)
	randBytes := make([]byte, 6)
	if _, err := rand.Read(randBytes); err != nil {
		return fmt.Errorf("generating temp name: %w", err)
	}
	tmpPath := filepath.Join(dir, ".tmp-otb-"+hex.EncodeToString(randBytes)+".md")

	content := strings.Join(lines, eol)
	if err := os.WriteFile(tmpPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		// Best-effort cleanup
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// resolveFilePath resolves task.SourceFile inside vaultPath with traversal guard.
func resolveFilePath(vaultPath string, task *parser.Task) (string, error) {
	abs, err := vault.IsUnderVault(vaultPath, task.SourceFile)
	if err != nil {
		return "", fmt.Errorf("invalid source file: %w", err)
	}
	return abs, nil
}

// verifyLine checks that the line at task.LineNumber still contains the
// expected task text, giving confidence we're mutating the right line.
func verifyLine(lines []string, task *parser.Task) error {
	idx := task.LineNumber - 1
	if idx < 0 || idx >= len(lines) {
		return fmt.Errorf("%w: line %d does not exist (file has %d lines)",
			ErrFingerprintMismatch, task.LineNumber, len(lines))
	}
	line := lines[idx]

	m := reCheckbox.FindStringSubmatch(line)
	if m == nil {
		return fmt.Errorf("%w: line %d is not a task line: %q",
			ErrFingerprintMismatch, task.LineNumber, line)
	}

	// Extract raw content after checkbox
	rest := m[2] // everything after "]"
	if len(rest) > 1 {
		rest = rest[1:] // strip leading space
	}
	actualNorm := normalizeLine(rest)
	expectedNorm := normalizeLine(task.Text)

	if actualNorm != expectedNorm {
		// Allow prefix match as fallback (file may have had minor edits)
		prefix := expectedNorm
		if len([]rune(prefix)) > 20 {
			prefix = string([]rune(prefix)[:20])
		}
		if len(prefix) < 5 || !strings.HasPrefix(actualNorm, prefix) {
			return fmt.Errorf("%w: expected %q, got %q at line %d",
				ErrFingerprintMismatch, expectedNorm, actualNorm, task.LineNumber)
		}
	}
	return nil
}

// ChangeTaskStatus updates the checkbox character for the given task.
func ChangeTaskStatus(vaultPath string, task *parser.Task, newStatus parser.TaskStatus) error {
	if task.Status == newStatus {
		return nil
	}
	filePath, err := resolveFilePath(vaultPath, task)
	if err != nil {
		return err
	}

	lines, eol, err := readLines(filePath)
	if err != nil {
		return err
	}

	if err := verifyLine(lines, task); err != nil {
		return err
	}

	idx := task.LineNumber - 1
	char := statusToChar[newStatus]
	updated := reCheckbox.ReplaceAllStringFunc(lines[idx], func(s string) string {
		m := reCheckbox.FindStringSubmatch(s)
		if m == nil {
			return s
		}
		return m[1] + char + m[2]
	})

	if updated == lines[idx] {
		return fmt.Errorf("could not locate checkbox on line %d", task.LineNumber)
	}
	lines[idx] = updated

	return writeLines(filePath, lines, eol)
}

// AddTaskComment appends a comment:: subtopic below the given task.
// author is sanitized to [a-zA-Z0-9_-].
func AddTaskComment(vaultPath string, task *parser.Task, text, author string) error {
	filePath, err := resolveFilePath(vaultPath, task)
	if err != nil {
		return err
	}

	lines, eol, err := readLines(filePath)
	if err != nil {
		return err
	}

	if err := verifyLine(lines, task); err != nil {
		return err
	}

	// Sanitize inputs
	safeText := sanitize.ForDisplay(text, 200)
	if safeText == "" {
		return errors.New("comment text is empty after sanitization")
	}
	safeAuthor := reAuthor.ReplaceAllString(author, "")
	if safeAuthor == "" {
		safeAuthor = "user"
	}

	now := time.Now()
	ts := fmt.Sprintf("%04d-%02d-%02d %02d:%02d",
		now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute())
	commentLine := fmt.Sprintf("  - comment:: %s @%s — %s", ts, safeAuthor, safeText)

	// Insert after existing comments for this task
	insertAt := task.LineNumber // 0-indexed insert point (after line idx)
	for insertAt < len(lines) && isCommentLine(lines[insertAt]) {
		insertAt++
	}

	// Splice commentLine in
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:insertAt]...)
	newLines = append(newLines, commentLine)
	newLines = append(newLines, lines[insertAt:]...)

	return writeLines(filePath, newLines, eol)
}

var reCommentLine = regexp.MustCompile(`^\s{2,}[-*]\s+comment::`)

func isCommentLine(s string) bool {
	return reCommentLine.MatchString(s)
}
