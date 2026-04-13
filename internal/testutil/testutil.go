// Package testutil provides shared helpers for unit and fuzz tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// MakeVault creates a minimal Obsidian vault structure inside a temp directory.
// Returns the vault root path.
func MakeVault(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".obsidian"), 0750); err != nil {
		t.Fatalf("MakeVault: %v", err)
	}
	return root
}

// MakeVaultWithFile creates a vault and writes content to relPath inside it.
// Returns the vault root path.
func MakeVaultWithFile(t testing.TB, relPath, content string) string {
	t.Helper()
	root := MakeVault(t)
	full := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0750); err != nil {
		t.Fatalf("MakeVaultWithFile mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0600); err != nil {
		t.Fatalf("MakeVaultWithFile write: %v", err)
	}
	return root
}

// WriteFile writes content to path, creating parent directories.
func WriteFile(t testing.TB, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatalf("WriteFile mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// ReadFile reads a file and returns its contents.
func ReadFile(t testing.TB, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %q: %v", path, err)
	}
	return string(b)
}
