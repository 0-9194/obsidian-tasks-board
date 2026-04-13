// Package reader scans an Obsidian vault for Markdown files and aggregates
// tasks into a BoardData structure.
package reader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pot-labs/otb/internal/parser"
)

const maxFileSizeBytes = 10 * 1024 * 1024 // 10 MB

// Config controls which directories and files the reader scans.
type Config struct {
	// ScanDirs are directories to scan, relative to vault root.
	// Defaults to the otb default layout if nil.
	ScanDirs []string
	// Excluded is a set of basenames to skip (e.g. "Board Global.md").
	Excluded map[string]struct{}
}

var defaultScanDirs = []string{"20 - Projects", "docs"}
var defaultExcluded = map[string]struct{}{"Board Global.md": {}}

// ProjectSummary holds the tasks from a single Markdown file.
type ProjectSummary struct {
	Name         string
	File         string
	RelativePath string
	Tasks        []parser.Task
}

// BoardData is the aggregated view of all tasks across the vault.
type BoardData struct {
	Projects []ProjectSummary
	AllTasks []parser.Task
	ByStatus map[parser.TaskStatus][]parser.Task
}

// Read scans the vault at vaultPath and returns a BoardData.
func Read(vaultPath string, cfg *Config) (*BoardData, error) {
	scanDirs := defaultScanDirs
	excluded := defaultExcluded

	if cfg != nil {
		if len(cfg.ScanDirs) > 0 {
			scanDirs = cfg.ScanDirs
		}
		if cfg.Excluded != nil {
			excluded = cfg.Excluded
		}
	}

	var projects []ProjectSummary

	for _, dirName := range scanDirs {
		dir := filepath.Join(vaultPath, dirName)
		files, err := discoverFiles(dir, excluded)
		if err != nil {
			// Directory may not exist — skip silently
			continue
		}
		for _, filePath := range files {
			summary, err := loadFile(filePath, vaultPath)
			if err != nil {
				continue
			}
			projects = append(projects, *summary)
		}
	}

	allTasks := make([]parser.Task, 0)
	for _, p := range projects {
		allTasks = append(allTasks, p.Tasks...)
	}

	byStatus := map[parser.TaskStatus][]parser.Task{
		parser.StatusTodo:       {},
		parser.StatusInProgress: {},
		parser.StatusDone:       {},
		parser.StatusCancelled:  {},
	}
	for _, t := range allTasks {
		byStatus[t.Status] = append(byStatus[t.Status], t)
	}

	return &BoardData{
		Projects: projects,
		AllTasks: allTasks,
		ByStatus: byStatus,
	}, nil
}

// discoverFiles returns sorted .md file paths in dir, skipping excluded names
// and symlinks. Files larger than maxFileSizeBytes are skipped.
func discoverFiles(dir string, excluded map[string]struct{}) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading dir %q: %w", dir, err)
	}

	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if _, skip := excluded[name]; skip {
			continue
		}

		fullPath := filepath.Join(dir, name)

		// Security: skip symlinks
		lstat, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}
		if lstat.Mode()&fs.ModeSymlink != 0 {
			continue
		}

		// Guard: skip excessively large files
		if lstat.Size() > maxFileSizeBytes {
			continue
		}

		if lstat.Mode().IsRegular() {
			paths = append(paths, fullPath)
		}
	}

	sort.Strings(paths)
	return paths, nil
}

// loadFile reads and parses a single Markdown file.
func loadFile(filePath, vaultRoot string) (*ProjectSummary, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %q: %w", filePath, err)
	}

	rel := filePath
	if strings.HasPrefix(filePath, vaultRoot) {
		rel = strings.TrimPrefix(filePath, vaultRoot)
		rel = strings.TrimPrefix(rel, string(filepath.Separator))
	}

	tasks := parser.ParseProjectFile(string(data), rel)
	base := filepath.Base(filePath)
	name := strings.TrimSuffix(base, ".md")

	return &ProjectSummary{
		Name:         name,
		File:         base,
		RelativePath: rel,
		Tasks:        tasks,
	}, nil
}
