// Package sanitize strips hostile terminal sequences and unsafe Unicode
// from strings read out of vault files before they reach the TUI renderer
// or are written back to disk.
//
// Attack vectors mitigated:
//   - ANSI/VT CSI sequences  (cursor hijack, colour injection)
//   - OSC sequences          (title overwrite, hyperlink abuse)
//   - DCS/SOS/PM/APC seqs   (terminal state corruption)
//   - SS2/SS3 sequences
//   - RIS (full terminal reset)
//   - C0 control chars       (0x00–0x1F) and DEL (0x7F)
//   - C1 control chars       (0x80–0x9F) — covers 8-bit CSI/OSC/DCS
//   - Unicode bidi overrides (U+202A–202E, U+2066–2069, U+200E/F, U+FEFF)
//   - Null bytes
package sanitize

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Compiled once at package init — all patterns are anchored to specific
// byte sequences so they cannot be confused with printable content.
var (
	// CSI: ESC [ <params> <final>
	reCSI = regexp.MustCompile(`\x1b\[[\x20-\x3f]*[\x40-\x7e]`)
	// OSC: ESC ] ... BEL  or  ESC ] ... ST (ESC \)
	reOSC = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
	// DCS/SOS/PM/APC: ESC [P X ^ _] ... ST
	reSTS = regexp.MustCompile(`\x1b[PX\^_][^\x1b]*\x1b\\`)
	// SS2/SS3: ESC N/O + one char
	reSS = regexp.MustCompile(`\x1b[NO][\s\S]`)
	// RIS: ESC c
	reRIS = regexp.MustCompile(`\x1bc`)
	// Bare ESC (anything we missed above)
	reESC = regexp.MustCompile(`\x1b`)
)

// bidiRanges lists Unicode bidi override / isolation codepoints to strip.
// Using a lookup function is faster than a regex for this set.
func isBidi(r rune) bool {
	switch r {
	case 0x200E, 0x200F, // LRM, RLM
		0x202A, 0x202B, 0x202C, 0x202D, 0x202E, // LRE RLE PDF LRO RLO
		0x2066, 0x2067, 0x2068, 0x2069, // LRI RLI FSI PDI
		0xFEFF: // BOM / ZWNBSP
		return true
	}
	return false
}

// isC0orDEL returns true for C0 control chars (0x00–0x1F) and DEL (0x7F).
// We keep 0x09 (tab) and 0x0A (newline) because callers split on newlines
// before calling sanitize; individual lines should never contain them.
func isC0orDEL(r rune) bool {
	return r <= 0x1F || r == 0x7F
}

// isC1 returns true for the C1 control range (0x80–0x9F), which encodes
// 8-bit equivalents of ESC-[ and friends.
func isC1(r rune) bool {
	return r >= 0x80 && r <= 0x9F
}

// ForDisplay sanitizes raw vault text for safe rendering in a terminal TUI.
// It strips all terminal-injection sequences and unsafe Unicode, collapses
// runs of whitespace, and truncates to maxLength runes (appending "…").
//
// maxLength ≤ 0 means no length limit.
func ForDisplay(raw string, maxLength int) string {
	if raw == "" {
		return ""
	}

	s := raw

	// 1. Strip multi-byte VT sequences (order matters: most-specific first)
	s = reSTS.ReplaceAllString(s, "")
	s = reOSC.ReplaceAllString(s, "")
	s = reCSI.ReplaceAllString(s, "")
	s = reSS.ReplaceAllString(s, "")
	s = reRIS.ReplaceAllString(s, "")
	s = reESC.ReplaceAllString(s, "")

	// 2. Strip dangerous Unicode and C0/C1 controls rune-by-rune
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !utf8.ValidRune(r) || r == utf8.RuneError {
			continue
		}
		if isC0orDEL(r) || isC1(r) || isBidi(r) || unicode.Is(unicode.Cs, r) {
			continue
		}
		b.WriteRune(r)
	}
	s = b.String()

	// 3. Collapse runs of whitespace to a single space and trim
	s = strings.Join(strings.Fields(s), " ")

	// 4. Enforce maxLength (in runes, not bytes)
	if maxLength > 0 {
		runes := []rune(s)
		if len(runes) > maxLength {
			s = string(runes[:maxLength-1]) + "…"
		}
	}

	return s
}
