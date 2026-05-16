package rules

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP118 checks for numeric index access without a length guard that may panic on empty collections.
type SLP118 struct{}

func (SLP118) ID() string                { return "SLP118" }
func (SLP118) DefaultSeverity() Severity { return SeverityBlock }
func (SLP118) Description() string {
	return "numeric index access without length guard — may panic on empty collection (only detects numeric-literal index forms)"
}

var slp118IndexRe = regexp.MustCompile(`(?:[A-Za-z0-9_]|[\)\]\}])\s*\[\d+\]`)
var slp118IndexNumRe = regexp.MustCompile(`\[(\d+)\]`)
var slp118GoGuardRe = regexp.MustCompile(`len\((.+?)\)\s*>\s*(\d+)|len\((.+?)\)\s*>=\s*(\d+)`)
var slp118JSGuardRe = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)\.length\s*>\s*(\d+)|([A-Za-z_$][A-Za-z0-9_$]*)\.length\s*>=\s*(\d+)`)
var slp118PyGuardRe = regexp.MustCompile(`len\((.+?)\)\s*>\s*(\d+)|len\((.+?)\)\s*>=\s*(\d+)`)

// Early-exit emptiness guards: `len(x) == 0`, `len(x) < 1`, `len(x) <= 0`
// (Go/Python) and the `.length` equivalents plus `!x.length` (JS/TS).
var slp118EmptyLenRe = regexp.MustCompile(`len\(\s*([A-Za-z_][A-Za-z0-9_.]*)\s*\)\s*(?:==\s*0\b|<\s*1\b|<=\s*0\b)`)
var slp118EmptyJSRe = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)\??\.length\s*(?:===?\s*0\b|<\s*1\b|<=\s*0\b)`)
var slp118EmptyJSNotRe = regexp.MustCompile(`!\s*([A-Za-z_$][A-Za-z0-9_$]*)\??\.length\b`)
var slp118ExitRe = regexp.MustCompile(`\b(?:return|continue|break|panic|throw)\b`)

type slp118Guard struct {
	collection  string
	bound       int
	op          string
	startIndent int
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func slp118LeadingSpaces(s string) int {
	n := 0
	for _, c := range s {
		if c == ' ' || c == '\t' {
			n++
		} else {
			break
		}
	}
	return n
}

func slp118ExtractGoGuards(line string) []*slp118Guard {
	var guards []*slp118Guard
	matches := slp118GoGuardRe.FindAllStringSubmatch(line, -1)
	for _, m := range matches {
		if m[1] != "" {
			guards = append(guards, &slp118Guard{collection: m[1], bound: atoiSafe(m[2]), op: ">"})
		} else if m[3] != "" {
			guards = append(guards, &slp118Guard{collection: m[3], bound: atoiSafe(m[4]), op: ">="})
		}
	}
	return guards
}

func slp118ExtractJSGuards(line string) []*slp118Guard {
	var guards []*slp118Guard
	matches := slp118JSGuardRe.FindAllStringSubmatch(line, -1)
	for _, m := range matches {
		if m[1] != "" {
			guards = append(guards, &slp118Guard{collection: strings.TrimSpace(m[1]), bound: atoiSafe(m[2]), op: ">"})
		} else if m[3] != "" {
			guards = append(guards, &slp118Guard{collection: strings.TrimSpace(m[3]), bound: atoiSafe(m[4]), op: ">="})
		}
	}
	return guards
}

func slp118ExtractPyGuards(line string) []*slp118Guard {
	var guards []*slp118Guard
	matches := slp118PyGuardRe.FindAllStringSubmatch(line, -1)
	for _, m := range matches {
		if m[1] != "" {
			guards = append(guards, &slp118Guard{collection: m[1], bound: atoiSafe(m[2]), op: ">"})
		} else if m[3] != "" {
			guards = append(guards, &slp118Guard{collection: m[3], bound: atoiSafe(m[4]), op: ">="})
		}
	}
	return guards
}

func slp118ExtractGuards(line string, filePath string) []*slp118Guard {
	if isGoFile(filePath) {
		return slp118ExtractGoGuards(line)
	}
	if isJSOrTSFile(filePath) {
		return slp118ExtractJSGuards(line)
	}
	if isPythonFile(filePath) {
		return slp118ExtractPyGuards(line)
	}
	return nil
}

func slp118IsIndexSafeForGuard(guard *slp118Guard, idx int) bool {
	switch guard.op {
	case ">":
		return idx <= guard.bound
	case ">=":
		return idx < guard.bound
	default:
		return true
	}
}

func slp118CollectionOfAccess(content string, matchLoc []int) string {
	start := matchLoc[0]
	scanStart := start
	for scanStart > 0 {
		c := content[scanStart-1]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			scanStart--
		} else {
			break
		}
	}
	collectionEnd := start + 1
	for collectionEnd < len(content) {
		c := content[collectionEnd]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			collectionEnd++
		} else {
			break
		}
	}
	if scanStart < collectionEnd && collectionEnd <= len(content) {
		return content[scanStart:collectionEnd]
	}
	return ""
}

func slp118AllIndicesGuarded(guards []*slp118Guard, nonEmpty map[string]bool, content string) bool {
	locs := slp118IndexRe.FindAllStringIndex(content, -1)
	for _, loc := range locs {
		end := loc[1]
		if end < len(content) && isAlpha(content[end]) {
			continue
		}

		collection := slp118CollectionOfAccess(content, loc)

		segment := content[loc[0]:]
		idxMatch := slp118IndexNumRe.FindStringSubmatch(segment)
		if idxMatch == nil {
			continue
		}
		idx := atoiSafe(idxMatch[1])

		// An early-exit guard such as `if len(x) == 0 { return }`
		// proves x has at least one element, so x[0] is safe.
		if idx == 0 && collection != "" && nonEmpty[collection] {
			continue
		}

		guarded := false
		for _, guard := range guards {
			if guard.collection == collection && slp118IsIndexSafeForGuard(guard, idx) {
				guarded = true
				break
			}
		}
		if !guarded {
			return false
		}
	}
	return true
}

func slp118IsBlockEnd(content string) bool {
	trimmed := strings.TrimSpace(content)
	return trimmed == "}" || trimmed == "fi" || trimmed == "end"
}

func slp118IsIndexAccess(content string) bool {
	locs := slp118IndexRe.FindAllStringIndex(content, -1)
	for _, loc := range locs {
		end := loc[1]
		if end < len(content) && isAlpha(content[end]) {
			continue
		}
		return true
	}
	return false
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func slp118CheckAccess(content string, guards []*slp118Guard, nonEmpty map[string]bool) bool {
	if !slp118IsIndexAccess(content) {
		return false
	}
	if slp118AllIndicesGuarded(guards, nonEmpty, content) {
		return false
	}
	return true
}

// slp118EmptyCheckCollections returns the collection names that a line
// tests for emptiness (e.g. `len(x) == 0`, `x.length < 1`, `!x.length`).
func slp118EmptyCheckCollections(line string, isJS bool) []string {
	var cols []string
	if isJS {
		for _, m := range slp118EmptyJSRe.FindAllStringSubmatch(line, -1) {
			cols = append(cols, m[1])
		}
		for _, m := range slp118EmptyJSNotRe.FindAllStringSubmatch(line, -1) {
			cols = append(cols, m[1])
		}
		return cols
	}
	for _, m := range slp118EmptyLenRe.FindAllStringSubmatch(line, -1) {
		cols = append(cols, m[1])
	}
	return cols
}

// slp118LookaheadExit reports whether one of the next few non-blank
// lines is a control-flow exit — the body of an early-exit guard.
func slp118LookaheadExit(stripped []string, start int) bool {
	checked := 0
	for i := start; i < len(stripped) && checked < 3; i++ {
		s := strings.TrimSpace(stripped[i])
		if s == "" || s == "{" {
			continue
		}
		checked++
		if slp118ExitRe.MatchString(s) {
			return true
		}
	}
	return false
}

// slp118NonEmptyAfterGuards returns collections proven non-empty by an
// early-exit emptiness guard such as `if len(x) == 0 { return }`. After
// such a guard x[0] cannot panic, so SLP118 must not flag it.
func slp118NonEmptyAfterGuards(lines []diff.Line, filePath string) map[string]bool {
	out := map[string]bool{}
	isJS := isJSOrTSFile(filePath)
	stripped := make([]string, len(lines))
	for i, ln := range lines {
		stripped[i] = stripCommentAndStrings(ln.Content)
	}
	for i, s := range stripped {
		cols := slp118EmptyCheckCollections(s, isJS)
		if len(cols) == 0 {
			continue
		}
		if !slp118ExitRe.MatchString(s) && !slp118LookaheadExit(stripped, i+1) {
			continue
		}
		for _, c := range cols {
			out[c] = true
		}
	}
	return out
}

func slp118IsCommentLine(content string) bool {
	return content == "" || strings.HasPrefix(content, "//") ||
		strings.HasPrefix(content, "/*") || strings.HasPrefix(content, "*") ||
		strings.HasPrefix(content, "#")
}

func (r SLP118) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) {
			continue
		}
		if !isGoFile(f.Path) && !isJSOrTSFile(f.Path) && !isPythonFile(f.Path) {
			continue
		}

		var allLines []diff.Line
		for _, h := range f.Hunks {
			allLines = append(allLines, h.Lines...)
		}
		nonEmpty := slp118NonEmptyAfterGuards(allLines, f.Path)

		for _, h := range f.Hunks {
			var currentGuards []*slp118Guard
			for _, ln := range h.Lines {
				rawIndent := slp118LeadingSpaces(ln.Content)
				stripped := stripCommentAndStrings(ln.Content)
				stripped = strings.TrimSpace(stripped)
				if stripped == "" {
					continue
				}

				if ln.Kind == diff.LineAdd {
					if slp118IsBlockEnd(stripped) {
						if len(currentGuards) > 0 && rawIndent <= currentGuards[0].startIndent {
							currentGuards = nil
						}
						continue
					}

					if len(currentGuards) > 0 && rawIndent <= currentGuards[0].startIndent {
						guards := slp118ExtractGuards(stripped, f.Path)
						if len(guards) == 0 {
							currentGuards = nil
						}
					}

					guards := slp118ExtractGuards(stripped, f.Path)
					if len(guards) > 0 {
						for _, g := range guards {
							g.startIndent = rawIndent
						}
						currentGuards = guards
					}

					if slp118IsCommentLine(stripped) {
						continue
					}

					if slp118CheckAccess(stripped, currentGuards, nonEmpty) {
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     ln.NewLineNo,
							Message:  "direct index access without length guard — may panic on empty collection",
							Snippet:  ln.Content,
						})
					}
				} else {
					guards := slp118ExtractGuards(stripped, f.Path)
					if len(guards) > 0 {
						for _, g := range guards {
							g.startIndent = rawIndent
						}
						currentGuards = guards
					}
				}
			}
		}
	}
	return out
}
