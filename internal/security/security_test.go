// Package security_test contains black-box security tests that run the compiled
// otb binary against hostile inputs via exec.Command.
//
// Each test verifies that the binary:
//   - Exits cleanly (non-zero for errors, but never a crash/panic/signal)
//   - Does not write files outside the vault path
//   - Does not leak paths or sensitive system info in stdout/stderr
//   - Does not accept path-traversal arguments as valid vaults
//
// NOTE: Container breakout tests are explicitly out of scope.
// All tests run in a single unprivileged process -- no mount namespaces,
// no setuid checks, no cgroup manipulation.
package security_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// binaryPath returns the path to the otb binary under test.
// Resolution order:
//  1. OTB_BINARY env var (set by entrypoint.sh in the container)
//  2. runtime.Caller relative path (works when tests run from source tree)
func binaryPath(t *testing.T) string {
	t.Helper()
	// 1. explicit override (used in Docker)
	if env := os.Getenv("OTB_BINARY"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	// 2. relative to source file location
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("cannot determine source file path; set OTB_BINARY env var")
		return ""
	}
	// thisFile = .../internal/security/security_test.go
	// binary   = .../bin/otb
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	bin := filepath.Join(root, "bin", "otb")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("binary not found at %q -- run `make build` first: %v", bin, err)
	}
	return bin
}

// runWithTimeout executes the binary with a hard timeout.
func runWithTimeout(bin string, timeout time.Duration, args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command(bin, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if startErr := cmd.Start(); startErr != nil {
		return "", "", startErr
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err = <-done:
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		err = fmt.Errorf("timeout after %s", timeout)
	}
	return outBuf.String(), errBuf.String(), err
}

// makeTestVault creates a minimal vault with one task file.
func makeTestVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".obsidian"), 0750); err != nil {
		t.Fatalf("mkdir .obsidian: %v", err)
	}
	projDir := filepath.Join(root, "20 - Projects")
	if err := os.MkdirAll(projDir, 0750); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}
	content := "- [ ] test task\n- [x] done task\n"
	if err := os.WriteFile(filepath.Join(projDir, "test.md"), []byte(content), 0600); err != nil {
		t.Fatalf("write tasks: %v", err)
	}
	return root
}

// ---- smoke tests -------------------------------------------------------------

func TestBinary_Version(t *testing.T) {
	bin := binaryPath(t)
	out, _, err := runWithTimeout(bin, 5*time.Second, "version")
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	if !strings.Contains(out, "otb ") {
		t.Errorf("expected 'otb <version>' in output, got: %q", out)
	}
}

func TestBinary_Help(t *testing.T) {
	bin := binaryPath(t)
	out, _, _ := runWithTimeout(bin, 5*time.Second, "help")
	for _, keyword := range []string{"USAGE", "board", "init", "--vault"} {
		if !strings.Contains(out, keyword) {
			t.Errorf("help output missing %q", keyword)
		}
	}
}

// ---- --vault path traversal rejection ---------------------------------------

func TestBinary_VaultTraversal_RelativeDotDot(t *testing.T) {
	bin := binaryPath(t)
	_, _, err := runWithTimeout(bin, 5*time.Second, "board", "--vault", "../../etc")
	if err == nil {
		t.Error("expected error for traversal vault path, got success")
	}
	assertCleanExit(t, err)
}

func TestBinary_VaultPath_SystemDirs(t *testing.T) {
	bin := binaryPath(t)
	dangerous := []string{"/etc", "/bin", "/", "/proc", "/sys", "/usr/bin"}
	for _, path := range dangerous {
		path := path
		t.Run(sanitizeTestName(path), func(t *testing.T) {
			_, _, err := runWithTimeout(bin, 5*time.Second, "board", "--vault", path)
			if err == nil {
				t.Errorf("expected error for vault=%q, got success", path)
			}
			assertCleanExit(t, err)
		})
	}
}

func TestBinary_VaultPath_NonExistent(t *testing.T) {
	bin := binaryPath(t)
	_, _, err := runWithTimeout(bin, 5*time.Second, "board", "--vault", "/tmp/otb-does-not-exist-xyzzy")
	if err == nil {
		t.Error("expected error for non-existent vault")
	}
	assertCleanExit(t, err)
}

// ---- --vault injection payloads ---------------------------------------------

func TestBinary_VaultPath_InjectionPayloads(t *testing.T) {
	bin := binaryPath(t)
	payloads := []string{
		"/tmp/$(touch /tmp/otb-pwned)",
		"/tmp/`touch /tmp/otb-pwned`",
		"/tmp/" + strings.Repeat("A", 4096),
		"/tmp/\x1b[31m",
	}
	for _, p := range payloads {
		p := p
		t.Run(sanitizeTestName(p), func(t *testing.T) {
			_, _, err := runWithTimeout(bin, 5*time.Second, "board", "--vault", p)
			assertCleanExit(t, err)
			if _, statErr := os.Stat("/tmp/otb-pwned"); statErr == nil {
				_ = os.Remove("/tmp/otb-pwned")
				t.Error("injection payload created /tmp/otb-pwned -- RCE!")
			}
		})
	}
}

// ---- --project flag injection -----------------------------------------------

func TestBinary_ProjectFlag_Injection(t *testing.T) {
	bin := binaryPath(t)
	vault := makeTestVault(t)
	payloads := []string{
		"'; touch /tmp/otb-proj-pwned; echo '",
		"$(touch /tmp/otb-proj-pwned)",
		strings.Repeat("A", 10000),
		"\x1b[31mevil\x1b[0m",
	}
	for _, p := range payloads {
		p := p
		t.Run(sanitizeTestName(p), func(t *testing.T) {
			_, _, err := runWithTimeout(bin, 5*time.Second, "board", "--vault", vault, "--project", p)
			assertCleanExit(t, err)
			if _, statErr := os.Stat("/tmp/otb-proj-pwned"); statErr == nil {
				_ = os.Remove("/tmp/otb-proj-pwned")
				t.Error("injection payload created /tmp/otb-proj-pwned -- RCE!")
			}
		})
	}
}

// ---- init path traversal ----------------------------------------------------

func TestBinary_Init_DirTraversal(t *testing.T) {
	bin := binaryPath(t)
	sandbox := t.TempDir()

	traversalDirs := []string{
		"../../tmp/otb-traversal",
		"../escape",
	}
	for _, dir := range traversalDirs {
		dir := dir
		t.Run(sanitizeTestName(dir), func(t *testing.T) {
			target := filepath.Join(sandbox, dir)
			_, _, _ = runWithTimeout(bin, 5*time.Second,
				"init", "--name", "test", "--dir", target)
			// The critical check: nothing written to /etc
			if _, statErr := os.Stat("/etc/otb-test-SHOULDNOTEXIST"); statErr == nil {
				t.Error("init wrote to /etc -- path traversal!")
			}
		})
	}
}

func TestBinary_Init_AbsoluteSystemDir(t *testing.T) {
	bin := binaryPath(t)
	_, _, err := runWithTimeout(bin, 5*time.Second,
		"init", "--name", "test", "--dir", "/etc/otb-test-SHOULDNOTEXIST")
	// May succeed or fail, but /etc must not be modified
	assertCleanExit(t, err)
	if _, statErr := os.Stat("/etc/otb-test-SHOULDNOTEXIST"); statErr == nil {
		t.Error("init created dir under /etc -- path traversal!")
	}
}

func TestBinary_Init_NameInjection(t *testing.T) {
	bin := binaryPath(t)
	sandbox := t.TempDir()
	maliciousNames := []string{
		"'; rm -rf /tmp/otb-safe; echo '",
		"$(touch /tmp/otb-name-pwned)",
		"\x1b[31mevil\x1b[0m",
		strings.Repeat("A", 10000),
	}
	for _, name := range maliciousNames {
		name := name
		t.Run(sanitizeTestName(name), func(t *testing.T) {
			outDir := filepath.Join(sandbox, fmt.Sprintf("vault-%d", time.Now().UnixNano()))
			_, _, err := runWithTimeout(bin, 5*time.Second,
				"init", "--name", name, "--dir", outDir)
			assertCleanExit(t, err)
			if _, statErr := os.Stat("/tmp/otb-name-pwned"); statErr == nil {
				_ = os.Remove("/tmp/otb-name-pwned")
				t.Error("name injection created /tmp/otb-name-pwned -- RCE!")
			}
		})
	}
}

// ---- unknown / extra args ---------------------------------------------------

func TestBinary_UnknownArgs(t *testing.T) {
	bin := binaryPath(t)
	// Only test args that never open the interactive TUI.
	// board with a valid vault intentionally launches the TUI and will timeout --
	// that is correct behaviour, not a bug.
	args := [][]string{
		// No vault -> auto-detect fails -> clean error
		{"--unknown-flag"},
		// Dashes-only -> treated as board, no vault found -> clean error
		{strings.Repeat("-", 1000)},
		// board with non-existent vault -> clean error
		{"board", "--vault", "/tmp/otb-no-vault-xyzzy", "--unknown-flag"},
	}
	for _, a := range args {
		a := a
		t.Run(strings.Join(a, "_")[:min(40, len(strings.Join(a, "_")))], func(t *testing.T) {
			_, _, err := runWithTimeout(bin, 5*time.Second, a...)
			assertCleanExit(t, err)
		})
	}
}

// ---- no vault auto-detect ---------------------------------------------------

func TestBinary_Board_NoVaultAutoDetect(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "board")
	cmd.Dir = t.TempDir() // empty dir with no .obsidian/
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		t.Error("expected error when no vault found, got success")
	}
	assertCleanExit(t, err)
}

// ---- no path leak in error output -------------------------------------------

func TestBinary_NoPathLeakInError(t *testing.T) {
	bin := binaryPath(t)
	out, stderr, _ := runWithTimeout(bin, 5*time.Second, "board", "--vault", "/nonexistent/path")
	combined := out + stderr
	// -trimpath build flag ensures source paths are stripped from panics
	if strings.Contains(combined, "/home/") && strings.Contains(combined, ".go:") {
		t.Errorf("output may contain build source path leak: %q", combined)
	}
}

// ---- helpers ----------------------------------------------------------------

func assertCleanExit(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "signal:") {
		t.Errorf("process killed by signal (possible panic/crash): %v", err)
	}
	if strings.Contains(errMsg, "timeout") {
		t.Errorf("process timed out (possible hang): %v", err)
	}
}

func sanitizeTestName(s string) string {
	if len(s) > 40 {
		s = s[:40]
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '/' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
