// Package vault resolves the absolute, canonical path to an Obsidian vault.
//
// Detection order (first match wins):
//  1. Explicit --vault argument — returned as-is after EvalSymlinks + validation
//  2. cwd itself has .obsidian/ — cwd IS the vault
//  3. Exactly one child of cwd has .obsidian/ — that child is the vault
//
// The presence of a .obsidian/ directory is the canonical marker for an
// Obsidian vault. No vault names are hardcoded.
package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is returned when no vault can be located.
var ErrNotFound = errors.New("no Obsidian vault found")

// ErrAmbiguous is returned when multiple vault candidates are found.
var ErrAmbiguous = errors.New("multiple Obsidian vaults found; use --vault to specify one")

// ErrTraversal is returned when a supplied path escapes the expected boundary.
var ErrTraversal = errors.New("path traversal detected in vault path")

func hasObsidian(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".obsidian"))
	return err == nil && info.IsDir()
}

// Resolve returns the absolute, symlink-resolved path to an Obsidian vault.
//
// If explicit is non-empty it is validated and returned directly.
// Otherwise cwd and its direct children are searched.
func Resolve(cwd, explicit string) (string, error) {
	if explicit != "" {
		return resolveExplicit(explicit)
	}

	// Canonicalize cwd first
	cwdAbs, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		cwdAbs = filepath.Clean(cwd)
	}

	// Case 1: cwd is the vault
	if hasObsidian(cwdAbs) {
		return cwdAbs, nil
	}

	// Case 2: single child
	entries, err := os.ReadDir(cwdAbs)
	if err != nil {
		return "", fmt.Errorf("reading directory %q: %w", cwdAbs, err)
	}

	var candidates []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(cwdAbs, e.Name())
		if hasObsidian(candidate) {
			candidates = append(candidates, candidate)
		}
	}

	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		return "", fmt.Errorf("%w\n  searched: %q\n\n  solutions:\n    • run from inside the vault (must contain .obsidian/)\n    • run from a directory that contains exactly one vault\n    • pass --vault /path/to/vault", ErrNotFound, cwdAbs)
	default:
		return "", fmt.Errorf("%w in %q", ErrAmbiguous, cwdAbs)
	}
}

// resolveExplicit validates and canonicalises an explicitly-provided vault path.
// It guards against path-traversal by ensuring the resolved path still
// contains ".obsidian/" after symlink resolution.
func resolveExplicit(raw string) (string, error) {
	// EvalSymlinks also cleans the path, catching ../../ traversals
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("invalid vault path %q: %w", raw, err)
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Path doesn't exist yet or has broken links — use Abs as fallback
		canonical = abs
	}

	// Guard: the canonical path must not be "/" or a system directory
	canonical = filepath.Clean(canonical)
	if isSensitivePath(canonical) {
		return "", fmt.Errorf("%w: %q resolves to a system path", ErrTraversal, raw)
	}

	if !hasObsidian(canonical) {
		return "", fmt.Errorf("path %q is not an Obsidian vault (missing .obsidian/)", canonical)
	}
	return canonical, nil
}

// sensitiveRoots are directories that must never be used as vaults.
var sensitiveRoots = []string{"/", "/etc", "/bin", "/sbin", "/usr", "/lib", "/proc", "/sys"}

func isSensitivePath(p string) bool {
	clean := filepath.Clean(p)
	for _, root := range sensitiveRoots {
		if clean == root || strings.HasPrefix(clean, root+string(filepath.Separator)) {
			// Allow /usr/local/... or /home/... style paths that happen to
			// start with a prefix, but block exact dangerous roots.
			if clean == root {
				return true
			}
		}
	}
	// Extra check: if path ends up at filesystem root
	return clean == "/"
}

// IsUnderVault checks that a relative file path (from task.SourceFile) does not
// escape the vault root via traversal. Returns the absolute path if safe.
func IsUnderVault(vaultPath, relPath string) (string, error) {
	// Security: reject absolute paths — sourceFile must always be relative
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("%w: %q is an absolute path, expected relative", ErrTraversal, relPath)
	}

	abs := filepath.Join(vaultPath, relPath)
	abs = filepath.Clean(abs)

	vaultClean := filepath.Clean(vaultPath)
	if !strings.HasPrefix(abs, vaultClean+string(filepath.Separator)) && abs != vaultClean {
		return "", fmt.Errorf("%w: %q escapes vault root %q", ErrTraversal, relPath, vaultPath)
	}
	return abs, nil
}
