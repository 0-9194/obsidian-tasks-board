// Package sanitize_fuzz contains fuzz tests for the sanitize package.
package sanitize_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/pot-labs/otb/internal/sanitize"
)

// ── Fuzz: ForDisplay ──────────────────────────────────────────────────────────
//
// Invariants:
//  1. Never panics
//  2. Output is always valid UTF-8
//  3. Output contains no ESC byte (0x1b)
//  4. Output contains no C0 controls (0x00–0x1F) except may have been stripped
//  5. Output contains no C1 controls (0x80–0x9F)
//  6. Output contains no bidi override codepoints
//  7. Length ≤ maxLength runes when maxLength > 0
//  8. Empty input → empty output

func FuzzForDisplay(f *testing.F) {
	seeds := []string{
		"",
		"hello world",
		"task with [type:: feat]",
		"\x1b[31mred text\x1b[0m",
		"\x1b]0;evil title\x07",
		"\x1bP data \x1b\\",         // DCS
		"\x1b[1;2;3;4;5;6;7;8;9m",  // multi-param CSI
		"\x00\x01\x02\x03",          // C0
		"\x80\x81\x9e\x9f",          // C1
		"\u202e evil \u202c",         // bidi
		"\ufeff BOM",
		"<script>alert(1)</script>",
		strings.Repeat("A", 10000),
		strings.Repeat("界", 1000),
		"normal\x00with\x00nulls",
		"\x1bc",                      // RIS
		"\x1bN\x41",                  // SS2
	}
	for _, s := range seeds {
		f.Add(s, 300)
		f.Add(s, 0)
		f.Add(s, 1)
	}

	f.Fuzz(func(t *testing.T, raw string, maxLen int) {
		// Clamp maxLen to reasonable range to avoid trivial truncation noise
		if maxLen < 0 {
			maxLen = 0
		}
		if maxLen > 100_000 {
			maxLen = 100_000
		}

		out := sanitize.ForDisplay(raw, maxLen)

		// Invariant 1: valid UTF-8
		if !utf8.ValidString(out) {
			t.Errorf("output not valid UTF-8 for input %q", raw)
		}

		// Invariant 2: no ESC
		if strings.ContainsRune(out, '\x1b') {
			t.Errorf("output contains ESC for input %q", raw)
		}

		// Invariant 3: no C0 controls (0x00..0x1F)
		for i, r := range out {
			if r >= 0x00 && r <= 0x1F {
				t.Errorf("output[%d] contains C0 rune %U for input %q", i, r, raw)
			}
		}

		// Invariant 4: no C1 controls (0x80..0x9F)
		for i, r := range out {
			if r >= 0x80 && r <= 0x9F {
				t.Errorf("output[%d] contains C1 rune %U for input %q", i, r, raw)
			}
		}

		// Invariant 5: no bidi overrides
		bidiSet := map[rune]bool{
			0x200E: true, 0x200F: true,
			0x202A: true, 0x202B: true, 0x202C: true, 0x202D: true, 0x202E: true,
			0x2066: true, 0x2067: true, 0x2068: true, 0x2069: true,
			0xFEFF: true,
		}
		for i, r := range out {
			if bidiSet[r] {
				t.Errorf("output[%d] contains bidi rune %U for input %q", i, r, raw)
			}
		}

		// Invariant 6: length bounded
		if maxLen > 0 && len([]rune(out)) > maxLen {
			t.Errorf("output length %d exceeds maxLen %d for input %q",
				len([]rune(out)), maxLen, raw)
		}

		// Invariant 7: empty input → empty output
		if raw == "" && out != "" {
			t.Errorf("empty input produced non-empty output: %q", out)
		}
	})
}
