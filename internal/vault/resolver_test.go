package vault_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pot-labs/otb/internal/vault"
)

func makeVault(t *testing.T, base, name string) string {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(filepath.Join(dir, ".obsidian"), 0750); err != nil {
		t.Fatalf("mkdir .obsidian: %v", err)
	}
	return dir
}

func TestResolve_CwdIsVault(t *testing.T) {
	tmp := t.TempDir()
	vaultDir := makeVault(t, tmp, "my-vault")

	got, err := vault.Resolve(vaultDir, "")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(vaultDir) {
		t.Errorf("got %q want %q", got, vaultDir)
	}
}

func TestResolve_SingleChildVault(t *testing.T) {
	tmp := t.TempDir()
	vaultDir := makeVault(t, tmp, "vault")

	got, err := vault.Resolve(tmp, "")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(vaultDir) {
		t.Errorf("got %q want %q", got, vaultDir)
	}
}

func TestResolve_AmbiguousVaults(t *testing.T) {
	tmp := t.TempDir()
	makeVault(t, tmp, "vault-a")
	makeVault(t, tmp, "vault-b")

	_, err := vault.Resolve(tmp, "")
	if err == nil {
		t.Fatal("expected error for ambiguous vaults, got nil")
	}
}

func TestResolve_NoVaultFound(t *testing.T) {
	tmp := t.TempDir()
	_, err := vault.Resolve(tmp, "")
	if err == nil {
		t.Fatal("expected error for no vault, got nil")
	}
}

func TestResolve_ExplicitPath(t *testing.T) {
	tmp := t.TempDir()
	vaultDir := makeVault(t, tmp, "explicit")

	got, err := vault.Resolve("/irrelevant", vaultDir)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(vaultDir) {
		t.Errorf("got %q want %q", got, vaultDir)
	}
}

func TestResolve_ExplicitPathNotVault(t *testing.T) {
	tmp := t.TempDir()
	notVault := filepath.Join(tmp, "not-a-vault")
	if err := os.MkdirAll(notVault, 0750); err != nil {
		t.Fatal(err)
	}

	_, err := vault.Resolve("", notVault)
	if err == nil {
		t.Fatal("expected error for non-vault path, got nil")
	}
}

// Security: path traversal in explicit --vault
func TestResolve_PathTraversalExplicit(t *testing.T) {
	tmp := t.TempDir()
	vaultDir := makeVault(t, tmp, "vault")

	// Construct a path with traversal that resolves outside the vault
	traversal := filepath.Join(vaultDir, "..", "..", "etc")
	_, err := vault.Resolve("", traversal)
	if err == nil {
		t.Fatalf("expected error for traversal path %q, got nil", traversal)
	}
}

// Security: IsUnderVault rejects escape
func TestIsUnderVault_Traversal(t *testing.T) {
	tmp := t.TempDir()
	vaultDir := makeVault(t, tmp, "vault")

	cases := []string{
		"../../etc/passwd",
		"../outside.md",
		"/etc/passwd",
	}
	for _, rel := range cases {
		_, err := vault.IsUnderVault(vaultDir, rel)
		if err == nil {
			t.Errorf("expected traversal error for %q, got nil", rel)
		}
	}
}

func TestIsUnderVault_ValidPath(t *testing.T) {
	tmp := t.TempDir()
	vaultDir := makeVault(t, tmp, "vault")

	abs, err := vault.IsUnderVault(vaultDir, "20 - Projects/my-project.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if abs == "" {
		t.Error("expected non-empty abs path")
	}
}
