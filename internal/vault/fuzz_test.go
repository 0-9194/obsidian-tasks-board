// Package vault_fuzz contains fuzz tests for the vault resolver.
package vault_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pot-labs/otb/internal/vault"
)

// ── Fuzz: Resolve ─────────────────────────────────────────────────────────────
//
// Invariants:
//  1. Never panics for any explicit path argument
//  2. If Resolve succeeds, the returned path must contain .obsidian/
//  3. System paths (/etc, /bin, /, /proc, /sys, /lib, /sbin) must always error
//  4. Traversal paths must error

func FuzzVaultResolve(f *testing.F) {
	seeds := []string{
		"",
		"/tmp",
		"/etc/passwd",
		"/",
		"../../etc/shadow",
		"../",
		strings.Repeat("../", 40) + "etc",
		"/proc/1",
		"/sys/class",
		"/bin/sh",
		"/dev/null",
		"\x00",
		"valid-but-missing-vault",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	// Dangerous system paths that must ALWAYS be rejected
	dangerousPrefixes := []string{
		"/etc", "/bin", "/sbin", "/lib", "/proc", "/sys",
	}

	f.Fuzz(func(t *testing.T, explicit string) {
		cwd := t.TempDir()

		result, err := vault.Resolve(cwd, explicit)

		if err != nil {
			// Errors are expected and correct for most fuzz inputs
			return
		}

		// If Resolve succeeded, the path must contain .obsidian/
		obsidianDir := filepath.Join(result, ".obsidian")
		info, statErr := os.Stat(obsidianDir)
		if statErr != nil || !info.IsDir() {
			t.Errorf("Resolve returned %q but .obsidian/ does not exist there", result)
		}

		// Guard: result must not be a dangerous system path
		clean := filepath.Clean(result)
		for _, prefix := range dangerousPrefixes {
			if clean == prefix || strings.HasPrefix(clean, prefix+string(filepath.Separator)) {
				t.Errorf("Resolve returned dangerous system path %q (prefix %q)", result, prefix)
			}
		}
	})
}

// ── Fuzz: IsUnderVault ────────────────────────────────────────────────────────
//
// Invariants:
//  1. Never panics
//  2. If it succeeds, the abs path starts with vaultPath
//  3. Absolute relPath inputs must always error
//  4. Traversal sequences must error when they escape the vault

func FuzzIsUnderVault(f *testing.F) {
	seeds := []string{
		"20 - Projects/tasks.md",
		"docs/ADR-001.md",
		"../escape.md",
		"../../etc/passwd",
		"/absolute.md",
		"./valid.md",
		"a/b/c/d/e.md",
		strings.Repeat("../", 30) + "etc/passwd",
		"\x00",
		"",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, relPath string) {
		vaultPath := t.TempDir()

		abs, err := vault.IsUnderVault(vaultPath, relPath)
		if err != nil {
			// Error for absolute or traversal paths is correct
			return
		}

		// Invariant: result must be under vault
		vaultClean := filepath.Clean(vaultPath)
		absClean := filepath.Clean(abs)
		if !strings.HasPrefix(absClean, vaultClean+string(filepath.Separator)) && absClean != vaultClean {
			t.Errorf("IsUnderVault returned %q which is outside vault %q (relPath=%q)",
				abs, vaultPath, relPath)
		}

		// Invariant: absolute relPath must have errored
		if filepath.IsAbs(relPath) {
			t.Errorf("IsUnderVault succeeded for absolute relPath=%q", relPath)
		}
	})
}
