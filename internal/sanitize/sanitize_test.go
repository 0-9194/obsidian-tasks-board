package sanitize_test

import (
	"strings"
	"testing"

	"github.com/pot-labs/otb/internal/sanitize"
)

func TestForDisplay_NormalText(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello world", "hello world"},
		{"  spaces   collapsed  ", "spaces collapsed"},
		{"task [type:: technical]", "task [type:: technical]"},
	}
	for _, c := range cases {
		got := sanitize.ForDisplay(c.in, 0)
		if got != c.want {
			t.Errorf("ForDisplay(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestForDisplay_CSIInjection(t *testing.T) {
	// CSI colour code, cursor movement
	cases := []string{
		"\x1b[31mred\x1b[0m",
		"\x1b[2J\x1b[H",  // clear screen + home
		"\x1b[?25l",       // hide cursor
		"prefix\x1b[1;32mgreen\x1b[0msuffix",
	}
	for _, in := range cases {
		got := sanitize.ForDisplay(in, 0)
		if strings.Contains(got, "\x1b") {
			t.Errorf("CSI not stripped: ForDisplay(%q) = %q", in, got)
		}
	}
}

func TestForDisplay_OSCInjection(t *testing.T) {
	cases := []string{
		"\x1b]0;evil title\x07",
		"\x1b]8;;http://evil.example\x07click\x1b]8;;\x07",
		"\x1b]0;pwned\x1b\\",
	}
	for _, in := range cases {
		got := sanitize.ForDisplay(in, 0)
		if strings.Contains(got, "\x1b") {
			t.Errorf("OSC not stripped: ForDisplay(%q) = %q", in, got)
		}
	}
}

func TestForDisplay_DCSInjection(t *testing.T) {
	cases := []string{
		"\x1bPdevil\x1b\\",
		"\x1b^SOS\x1b\\",
		"\x1b_APC\x1b\\",
		"\x1bXSOS\x1b\\",
	}
	for _, in := range cases {
		got := sanitize.ForDisplay(in, 0)
		if strings.Contains(got, "\x1b") {
			t.Errorf("DCS/SOS not stripped: ForDisplay(%q) = %q", in, got)
		}
	}
}

func TestForDisplay_C0Controls(t *testing.T) {
	cases := []string{
		"bell\x07",
		"null\x00byte",
		"back\x08space",
		"carriage\x0Dreturn",
		"formfeed\x0C",
		"\x01\x02\x03\x04\x05\x06",
	}
	for _, in := range cases {
		got := sanitize.ForDisplay(in, 0)
		for _, r := range got {
			if r <= 0x1F || r == 0x7F {
				t.Errorf("C0/DEL not stripped: ForDisplay(%q) = %q (contains 0x%02X)", in, got, r)
			}
		}
	}
}

func TestForDisplay_C1Controls(t *testing.T) {
	// 8-bit CSI (0x9B), 8-bit OSC (0x9D), etc.
	cases := []string{
		"\x9b31m", // 8-bit CSI ESC[31m equivalent
		"\x9d0;title\x07",
	}
	for _, in := range cases {
		got := sanitize.ForDisplay(in, 0)
		for _, r := range got {
			if r >= 0x80 && r <= 0x9F {
				t.Errorf("C1 not stripped: ForDisplay(%q) = %q", in, got)
			}
		}
	}
}

func TestForDisplay_BidiInjection(t *testing.T) {
	// Unicode bidi override chars
	cases := []string{
		"safe\u202Eevil",   // RLO
		"safe\u202Aevil",   // LRE
		"safe\u202Bevil",   // RLE
		"safe\u200Fevil",   // RLM
		"safe\uFEFFevil",   // BOM
		"safe\u2066evil",   // LRI
		"safe\u2069evil",   // PDI
	}
	bidiRunes := []rune{0x202A, 0x202B, 0x202C, 0x202D, 0x202E,
		0x2066, 0x2067, 0x2068, 0x2069, 0x200E, 0x200F, 0xFEFF}
	for _, in := range cases {
		got := sanitize.ForDisplay(in, 0)
		for _, bad := range bidiRunes {
			if strings.ContainsRune(got, bad) {
				t.Errorf("bidi rune U+%04X not stripped: ForDisplay(%q) = %q", bad, in, got)
			}
		}
	}
}

func TestForDisplay_NullBytes(t *testing.T) {
	in := "hello\x00world"
	got := sanitize.ForDisplay(in, 0)
	if strings.ContainsRune(got, 0) {
		t.Errorf("null byte not stripped: got %q", got)
	}
	if got != "helloworld" {
		t.Errorf("unexpected result: %q", got)
	}
}

func TestForDisplay_MaxLength(t *testing.T) {
	long := strings.Repeat("a", 300)
	got := sanitize.ForDisplay(long, 50)
	runes := []rune(got)
	if len(runes) > 50 {
		t.Errorf("maxLength not enforced: len=%d > 50", len(runes))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated string should end with ellipsis: %q", got)
	}
}

func TestForDisplay_MaxLengthZeroUnlimited(t *testing.T) {
	long := strings.Repeat("a", 500)
	got := sanitize.ForDisplay(long, 0)
	if len([]rune(got)) != 500 {
		t.Errorf("maxLength=0 should be unlimited, got len=%d", len([]rune(got)))
	}
}

func TestForDisplay_EmptyString(t *testing.T) {
	if got := sanitize.ForDisplay("", 100); got != "" {
		t.Errorf("empty input should return empty, got %q", got)
	}
}

func TestForDisplay_CombinedAttack(t *testing.T) {
	// Simulate a malicious vault task line
	in := "\x1b[31m\u202Emalicious\x00\x1b]0;pwned\x07 task\x1b[0m"
	got := sanitize.ForDisplay(in, 200)
	if strings.Contains(got, "\x1b") {
		t.Errorf("escape not stripped in combined attack: %q", got)
	}
	for _, r := range got {
		if r <= 0x1F || r == 0x7F {
			t.Errorf("control char 0x%02X not stripped in combined attack", r)
		}
	}
}
