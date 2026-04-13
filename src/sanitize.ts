/**
 * sanitize.ts — Security module
 *
 * Sanitizes strings read from repository files before rendering in the TUI.
 *
 * Attack vectors mitigated:
 *   - ANSI/VT escape sequence injection  (terminal hijacking, cursor manipulation)
 *   - OSC sequence injection             (title overwrite, hyperlink abuse)
 *   - DCS/SOS/PM/APC sequence injection  (terminal state corruption)
 *   - Unicode bidi override injection    (text spoofing via RLO/LRO)
 *   - Unicode bidi isolation injection   (FSI/LRI/RLI/PDI)
 *   - C0/C1 control character injection  (BEL, BS, CR, LF, DEL, etc.)
 *   - Null byte injection
 */

const RE_CSI     = /\x1b\[[\x20-\x3f]*[\x40-\x7e]/g;
const RE_OSC     = /\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g;
const RE_STS     = /\x1b[PX^_][^\x1b]*\x1b\\/g;
const RE_SS      = /\x1b[NO][\s\S]/g;
const RE_RIS     = /\x1bc/g;
const RE_ESC_BARE = /\x1b/g;
const RE_C0_DEL  = /[\x00-\x1f\x7f]/g;
const RE_BIDI    = /[\u202a-\u202e\u2066-\u2069\u200e\u200f\ufeff]/g;

export function sanitizeForDisplay(raw: unknown, maxLength = 200): string {
  if (typeof raw !== "string") return "";
  let s = raw;
  s = s.replace(RE_STS, "");
  s = s.replace(RE_OSC, "");
  s = s.replace(RE_CSI, "");
  s = s.replace(RE_SS, "");
  s = s.replace(RE_RIS, "");
  s = s.replace(RE_ESC_BARE, "");
  s = s.replace(RE_C0_DEL, "");
  s = s.replace(RE_BIDI, "");
  s = s.replace(/\s+/g, " ").trim();
  if (s.length > maxLength) s = s.slice(0, maxLength - 1) + "…";
  return s;
}
