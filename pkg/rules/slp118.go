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

func slp118AllIndicesGuarded(guards []*slp118Guard, nonEmpty map[string]int, content string) bool {
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
		if idx == 0 && collection != "" {
			if _, ok := nonEmpty[collection]; ok {
				continue
			}
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

func slp118CheckAccess(content string, guards []*slp118Guard, nonEmpty map[string]int) bool {
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
	var res []string
	if isJS {
		for _, m := range slp118EmptyJSRe.FindAllStringSubmatch(line, -1) {
			if len(m) > 1 {
				res = append(res, m[1])
			}
		}
		for _, m := range slp118EmptyJSNotRe.FindAllStringSubmatch(line, -1) {
			if len(m) > 1 {
				res = append(res, m[1])
			}
		}
		return res
	}
	for _, m := range slp118EmptyLenRe.FindAllStringSubmatch(line, -1) {
		if len(m) > 1 {
			res = append(res, m[1])
		}
	}
	return res
}

// slp118EarlyGuard records an early-exit emptiness guard such as
// `if len(x) == 0 { return }`. After such a guard x has at least one
// element, so x[0] is safe for the remainder of the enclosing block.
type slp118EarlyGuard struct {
	collection  string
	indent      int // indentation of the `if`
	afterLineNo int // active once the new-file line with this number is reached
}

// slp118IsIfLine reports whether a stripped line opens a conditional.
func slp118IsIfLine(s string) bool {
	if strings.HasPrefix(s, "if ") || strings.HasPrefix(s, "if(") {
		return true
	}
	return strings.HasPrefix(s, "} else if") || strings.HasPrefix(s, "else if")
}

// slp118SingleExitBlock inspects the block opened at openIdx. If the
// body is exactly one control-flow exit it returns the index of the
// block's final line and true. Brace blocks end at a `}` aligned with
// the opener; indentation blocks end on dedent. Requiring a sole exit
// statement prevents `if len(x) == 0 { log() }` from being mistaken for
// an early-exit guard. Detection bails if the scan crosses a diff hunk
// boundary (a gap in new-file line numbers) so a guard is never
// inferred across elided, unseen code.
func slp118SingleExitBlock(stripped []string, indents []int, lineNos []int, openIdx int) (int, bool) {
	openIndent := indents[openIdx]
	brace := strings.HasSuffix(stripped[openIdx], "{")
	bodyCount := 0
	bodyIsExit := false
	lastBody := openIdx
	for j := openIdx + 1; j < len(stripped); j++ {
		// A gap in new-file line numbers means an elided hunk boundary.
		if lineNos[j] == 0 || lineNos[j] != lineNos[j-1]+1 {
			return 0, false
		}
		s := stripped[j]
		if s == "" {
			continue
		}
		if brace {
			if s == "}" && indents[j] == openIndent {
				if bodyCount == 1 && bodyIsExit {
					return j, true
				}
				return 0, false
			}
		} else if indents[j] <= openIndent {
			break
		}
		bodyCount++
		if bodyCount > 1 {
			return 0, false
		}
		bodyIsExit = slp118ExitRe.MatchString(s)
		lastBody = j
	}
	if !brace && bodyCount == 1 && bodyIsExit {
		return lastBody, true
	}
	return 0, false
}

// slp118EarlyExitGuards scans the new-file view of a file (all hunks
// concatenated, deletions removed) for early-exit emptiness guards,
// returning each with the new-file line number after which it applies.
func slp118EarlyExitGuards(lines []diff.Line, filePath string) []slp118EarlyGuard {
	isJS := isJSOrTSFile(filePath)
	stripped := make([]string, len(lines))
	indents := make([]int, len(lines))
	lineNos := make([]int, len(lines))
	for i, ln := range lines {
		indents[i] = slp118LeadingSpaces(ln.Content)
		stripped[i] = strings.TrimSpace(stripCommentAndStrings(ln.Content))
		lineNos[i] = ln.NewLineNo
	}

	var guards []slp118EarlyGuard
	for i, s := range stripped {
		if s == "" || !slp118IsIfLine(s) {
			continue
		}
		cols := slp118EmptyCheckCollections(s, isJS)
		if len(cols) == 0 {
			continue
		}

		afterIdx := -1
		if slp118ExitRe.MatchString(s) {
			// Single-line form: `if len(x) == 0 { return }`.
			afterIdx = i
		} else if strings.HasSuffix(s, "{") || strings.HasSuffix(s, ":") {
			if end, ok := slp118SingleExitBlock(stripped, indents, lineNos, i); ok {
				afterIdx = end
			}
		}
		if afterIdx < 0 || lineNos[afterIdx] == 0 {
			continue
		}
		for _, c := range cols {
			guards = append(guards, slp118EarlyGuard{collection: c, indent: indents[i], afterLineNo: lineNos[afterIdx]})
		}
	}
	return guards
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

		// Concatenate every hunk's new-file lines (context + additions)
		// so an early-exit guard in one hunk is still recognised when the
		// access it protects appears in a later hunk of the same file.
		var newFileLines []diff.Line
		for _, h := range f.Hunks {
			for _, ln := range h.Lines {
				if ln.Kind != diff.LineDelete {
					newFileLines = append(newFileLines, ln)
				}
			}
		}
		early := slp118EarlyExitGuards(newFileLines, f.Path)

		// nonEmpty maps a collection to the indentation at which an
		// early-exit guard proved it non-empty; entries are dropped once
		// execution dedents out of that block. It persists across hunks.
		nonEmpty := map[string]int{}

		for _, h := range f.Hunks {
			var currentGuards []*slp118Guard

			for _, ln := range h.Lines {
				rawIndent := slp118LeadingSpaces(ln.Content)
				stripped := strings.TrimSpace(stripCommentAndStrings(ln.Content))

				if stripped != "" {
					for col, ind := range nonEmpty {
						if rawIndent < ind {
							delete(nonEmpty, col)
						}
					}

					if ln.Kind == diff.LineAdd {
						if slp118IsBlockEnd(stripped) {
							if len(currentGuards) > 0 && rawIndent <= currentGuards[0].startIndent {
								currentGuards = nil
							}
						} else {
							if len(currentGuards) > 0 && rawIndent <= currentGuards[0].startIndent {
								if len(slp118ExtractGuards(stripped, f.Path)) == 0 {
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

							if !slp118IsCommentLine(stripped) && slp118CheckAccess(stripped, currentGuards, nonEmpty) {
								out = append(out, Finding{
									RuleID:   r.ID(),
									Severity: r.DefaultSeverity(),
									File:     f.Path,
									Line:     ln.NewLineNo,
									Message:  "direct index access without length guard — may panic on empty collection",
									Snippet:  ln.Content,
								})
							}
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

				// Activate early-exit guards whose block ends on this line.
				if ln.NewLineNo != 0 {
					for _, g := range early {
						if g.afterLineNo == ln.NewLineNo {
							nonEmpty[g.collection] = g.indent
						}
					}
				}
			}
		}
	}
	return out
}
