package reader_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pot-labs/otb/internal/parser"
	"github.com/pot-labs/otb/internal/reader"
)

func setupVault(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	dirs := []string{
		".obsidian",
		"20 - Projects",
		"docs",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0750); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	return tmp
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestReader_DiscoversMDFiles(t *testing.T) {
	vault := setupVault(t)
	proj := filepath.Join(vault, "20 - Projects")

	writeFile(t, proj, "project-a.md", "- [ ] Task A\n")
	writeFile(t, proj, "project-b.md", "- [/] Task B\n")
	writeFile(t, proj, "notes.txt", "not markdown\n")

	data, err := reader.Read(vault, nil)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if len(data.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(data.Projects))
	}
}

func TestReader_SkipsExcluded(t *testing.T) {
	vault := setupVault(t)
	proj := filepath.Join(vault, "20 - Projects")

	writeFile(t, proj, "Board Global.md", "- [ ] Global task\n")
	writeFile(t, proj, "real-project.md", "- [ ] Real task\n")

	data, err := reader.Read(vault, nil)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	for _, p := range data.Projects {
		if p.File == "Board Global.md" {
			t.Error("Board Global.md should be excluded")
		}
	}
}

func TestReader_SkipsSymlinks(t *testing.T) {
	vault := setupVault(t)
	proj := filepath.Join(vault, "20 - Projects")

	// Create a real file and a symlink to it
	real := writeFile(t, proj, "real.md", "- [ ] Real task\n")
	symlink := filepath.Join(proj, "link.md")
	if err := os.Symlink(real, symlink); err != nil {
		t.Skip("symlink creation not supported:", err)
	}

	data, err := reader.Read(vault, nil)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	for _, p := range data.Projects {
		if p.File == "link.md" {
			t.Error("symlink should be skipped")
		}
	}
}

func TestReader_SkipsLargeFiles(t *testing.T) {
	vault := setupVault(t)
	proj := filepath.Join(vault, "20 - Projects")

	// Create a file just over 10MB
	big := filepath.Join(proj, "huge.md")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	chunk := []byte(strings.Repeat("- [ ] task\n", 100))
	written := 0
	for written < 10*1024*1024+1 {
		n, _ := f.Write(chunk)
		written += n
	}
	f.Close()

	data, err := reader.Read(vault, nil)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	for _, p := range data.Projects {
		if p.File == "huge.md" {
			t.Error("file > 10MB should be skipped")
		}
	}
}

func TestReader_AggregatesByStatus(t *testing.T) {
	vault := setupVault(t)
	proj := filepath.Join(vault, "20 - Projects")

	content := strings.Join([]string{
		"- [ ] todo one",
		"- [ ] todo two",
		"- [/] in progress",
		"- [x] done",
		"- [-] cancelled",
	}, "\n")
	writeFile(t, proj, "mixed.md", content)

	data, err := reader.Read(vault, nil)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}

	checks := map[parser.TaskStatus]int{
		parser.StatusTodo:       2,
		parser.StatusInProgress: 1,
		parser.StatusDone:       1,
		parser.StatusCancelled:  1,
	}
	for status, want := range checks {
		got := len(data.ByStatus[status])
		if got != want {
			t.Errorf("status %q: got %d tasks, want %d", status, got, want)
		}
	}
}

func TestReader_CustomScanDirs(t *testing.T) {
	vault := setupVault(t)
	customDir := filepath.Join(vault, "custom")
	if err := os.MkdirAll(customDir, 0750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, customDir, "tasks.md", "- [ ] Custom task\n")

	cfg := &reader.Config{
		ScanDirs: []string{"custom"},
	}
	data, err := reader.Read(vault, cfg)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if len(data.AllTasks) != 1 {
		t.Errorf("expected 1 task from custom dir, got %d", len(data.AllTasks))
	}
}

func TestReader_EmptyVault(t *testing.T) {
	vault := setupVault(t)
	data, err := reader.Read(vault, nil)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if len(data.AllTasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(data.AllTasks))
	}
}

func TestReader_NonExistentScanDir(t *testing.T) {
	vault := setupVault(t)
	cfg := &reader.Config{
		ScanDirs: []string{"nonexistent-dir"},
	}
	// Should not error, just return empty
	data, err := reader.Read(vault, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.AllTasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(data.AllTasks))
	}
}

func TestReader_MultipleFiles_AllTasksCombined(t *testing.T) {
	vault := setupVault(t)
	proj := filepath.Join(vault, "20 - Projects")
	docs := filepath.Join(vault, "docs")

	writeFile(t, proj, "p1.md", "- [ ] proj task\n")
	writeFile(t, docs, "d1.md", "- [/] docs task\n")

	data, err := reader.Read(vault, nil)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if len(data.AllTasks) != 2 {
		t.Errorf("expected 2 total tasks, got %d", len(data.AllTasks))
	}
}

func TestReader_StressLoad(t *testing.T) {
	vault := setupVault(t)
	proj := filepath.Join(vault, "20 - Projects")

	// Create 50 files with 20 tasks each
	for i := 0; i < 50; i++ {
		var lines []string
		for j := 0; j < 20; j++ {
			lines = append(lines, fmt.Sprintf("- [ ] task %d-%d", i, j))
		}
		writeFile(t, proj, fmt.Sprintf("file%02d.md", i), strings.Join(lines, "\n"))
	}

	data, err := reader.Read(vault, nil)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if len(data.AllTasks) != 1000 {
		t.Errorf("expected 1000 tasks, got %d", len(data.AllTasks))
	}
}
