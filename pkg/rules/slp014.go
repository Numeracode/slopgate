package rules

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP014 flags debug prints (fmt.Println, console.log, print(, etc.)
// added in non-test, non-main, non-doc files.
//
// Rationale: print-to-stdout for debugging is the oldest AI-coding
// failure mode. The model adds a `fmt.Println("here")` to figure out
// why something broke, then commits it without cleanup. In tests and
// CLI entrypoints prints are legitimate; everywhere else they are slop.
type SLP014 struct{}

func (SLP014) ID() string                { return "SLP014" }
func (SLP014) DefaultSeverity() Severity { return SeverityBlock }
func (SLP014) Description() string {
	return "debug print statement added in a non-test, non-entrypoint file"
}

// debugPrintPatterns matches the most common per-language prints.
// Each pattern requires the call syntax to be present — "fmt.Println"
// inside a string or comment has its own filter below.
//
// We deliberately skip console.warn / console.error / console.info:
// those are almost always real error-logging calls, not leftover
// debugging output. console.log / console.debug / console.trace are
// the ones AI agents add mid-task and forget to remove.
var debugPrintPatterns = []*regexp.Regexp{
	// Go
	regexp.MustCompile(`\bfmt\.(Println|Printf|Print)\s*\(`),
	// TypeScript / JavaScript
	regexp.MustCompile(`\bconsole\.(log|debug|trace)\s*\(`),
	// Python
	regexp.MustCompile(`(^|\W)print\s*\(`),
	// Java
	regexp.MustCompile(`\bSystem\.(out|err)\.(println|printf|print)\s*\(`),
	// Rust
	regexp.MustCompile(`\b(println|eprintln|print|eprint)!\s*\(`),
	regexp.MustCompile(`\bdbg!\s*\(`),
}

// isSuppressedDebugFile reports whether the file path is a location
// where debug prints are legitimate: test files, the entire cmd/**
// tree (CLI entrypoints whose job is to print), docs, and scripts.
func isSuppressedDebugFile(path string) bool {
	base := filepath.Base(path)
	lower := strings.ToLower(path)

	// Go test files.
	if strings.HasSuffix(lower, "_test.go") {
		return true
	}
	// JS/TS test files by common conventions.
	if strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") {
		return true
	}
	// Python test files.
	if strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") {
		return true
	}
	// Java/Kotlin test files (JUnit convention: *Test.java, *Tests.java).
	if isJavaTestFile(path) {
		return true
	}
	// Rust test files.
	if isRustTestFile(path) {
		return true
	}
	// Doc files.
	if isDocFile(path) {
		return true
	}
	// Go CLI packages: anything under cmd/** is a command entrypoint
	// whose role is to print output for the user. Suppressing by
	// directory (not filename) keeps a `pkg/cli/cmd_foo.go` honest
	// while letting a real CLI subcommand at `cmd/tool/cmd_foo.go`
	// print freely.
	if strings.HasPrefix(lower, "cmd/") || strings.Contains(lower, "/cmd/") {
		return true
	}
	// Top-level main.go in a single-package repo.
	if base == "main.go" && !strings.Contains(strings.TrimSuffix(path, base), "/") {
		return true
	}
	// Shell / scripts directories.
	if strings.HasPrefix(lower, "scripts/") || strings.HasPrefix(lower, "script/") {
		return true
	}
	return false
}

// stripCommentAndStrings removes comments (both line-end `//` and
// inline `/* ... */` block comments) and the contents of all three
// common string-literal kinds (double-quoted, single-quoted,
// backtick/raw) from a line so the debug-print patterns can't match
// inside them. It is intentionally simple — multi-line strings and
// unclosed block comments are out of scope for a single-line linter,
// and perfect escape handling is unnecessary.
//
// The returned string may be shorter than the input because line
// comments truncate at the comment start. For rules that need
// byte-offset alignment with the original string, use
// maskCommentAndStrings instead.
func stripCommentAndStrings(s string) string {
	// Strip full-line comments first.
	trimmed := strings.TrimLeft(s, " \t")
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "* ") {
		return ""
	}

	var b strings.Builder
	var quote byte // 0 when not in a string; otherwise the opening quote char
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			// Raw strings (backtick) have no escape sequences.
			if quote != '`' && c == '\\' && i+1 < len(s) {
				i++ // skip the escaped char
				continue
			}
			if c == quote {
				quote = 0
				b.WriteByte(c)
			}
			continue
		}
		switch {
		case c == '"' || c == '\'' || c == '`':
			quote = c
			b.WriteByte(c)
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			// Line comment — discard rest of line.
			return b.String()
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			// Block comment — skip until closing */.
			i += 2
			for i < len(s)-1 {
				if s[i] == '*' && s[i+1] == '/' {
					i++ // advance past the '/'
					break
				}
				i++
			}
		case c == '#':
			return b.String()
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// maskCommentAndStrings replaces comments and string contents with spaces
// while preserving the exact byte length of the input. This ensures that
// regex match indices from the masked string map directly to positions in
// the original string. Callers can use FindStringIndex on the masked string
// and then use those offsets to extract from the original string.
//
//   - String contents (between quotes) become spaces; the quotes themselves
//     are preserved so the structural shape of the line stays recognizable.
//   - Line comments (// and #) and block comments (/* ... */) are replaced
//     with spaces from the comment start to end of line/end of comment.
//   - The result always has len(masked) == len(s).
func maskCommentAndStrings(s string) string {
	out := []byte(s)
	var quote byte
	for i := 0; i < len(out); i++ {
		c := out[i]
		if quote != 0 {
			if quote != '`' && c == '\\' && i+1 < len(out) {
				// Escape sequence — blank the escaped char.
				out[i+1] = ' '
				continue
			}
			if c == quote {
				quote = 0
				// Closing quote stays.
			} else {
				out[i] = ' '
			}
			continue
		}
		switch {
		case c == '"' || c == '\'' || c == '`':
			quote = c
			// Opening quote stays.
		case c == '/' && i+1 < len(out) && out[i+1] == '/':
			// Line comment — blank rest of line.
			for j := i; j < len(out); j++ {
				out[j] = ' '
			}
			return string(out)
		case c == '/' && i+1 < len(out) && out[i+1] == '*':
			// Block comment — blank until closing */.
			out[i] = ' '
			out[i+1] = ' '
			j := i + 2
			closed := false
			for j < len(out)-1 {
				if out[j] == '*' && out[j+1] == '/' {
					out[j] = ' '
					out[j+1] = ' '
					j += 2
					closed = true
					break
				}
				out[j] = ' '
				j++
			}
			if closed {
				i = j - 1 // -1 because the loop will i++
			} else {
				// No closing */ — blank rest.
				for k := i; k < len(out); k++ {
					out[k] = ' '
				}
				return string(out)
			}
		case c == '#':
			// Python/shell comment — blank rest of line.
			for j := i; j < len(out); j++ {
				out[j] = ' '
			}
			return string(out)
		}
	}
	return string(out)
}

// catchOpenerRe matches a JS/TS/Go/Java/Rust catch clause opener:
//
//	} catch (err) {       (same-line)
//	catch (err) {         (after a } on the prior line)
//	} catch {             (Java-style untyped catch — not Go-valid, but harmless)
//	catch                 (multi-line — opening brace on next line)
var catchOpenerRe = regexp.MustCompile(`\bcatch\s*[({]|\bcatch\s*$`)

func indentWidth(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' {
			n++
			continue
		}
		break
	}
	return n
}

// isInsideCatchHandler reports whether the line at idx within lines is
// inside a JS/TS/Go/Java/Rust catch block (or a Python except clause).
// Debug-print calls in catch handlers are almost always intentional error
// logging (`console.debug('fetch failed (non-fatal):', err)`), not leftover
// debugging output, so SLP014 should not fire on them.
//
// Brace-based languages: walk backwards counting braces. The first unmatched
// `{` is the enclosing block's opener; if its line (or the previous non-blank
// line for split `} catch (e)\n{`) carries the `catch` keyword, we're in a
// catch handler. Python: walk back to the first line at strictly lower
// indentation; if it starts with `except`, we're in an except clause.
//
// The walk is bounded by the hunk's own line slice — if the catch opener
// is outside the hunk's context window, the rule will keep firing, which
// is the safe degradation.
func isInsideCatchHandler(lines []diff.Line, idx int, python bool) bool {
	if python {
		target := indentWidth(lines[idx].Content)
		for j := idx - 1; j >= 0; j-- {
			stripped := strings.TrimLeft(lines[j].Content, " \t")
			if stripped == "" || strings.HasPrefix(stripped, "#") {
				continue
			}
			if indentWidth(lines[j].Content) >= target {
				continue
			}
			return strings.HasPrefix(stripped, "except")
		}
		return false
	}
	depth := 0
	for j := idx - 1; j >= 0; j-- {
		masked := maskCommentAndStrings(lines[j].Content)
		for k := len(masked) - 1; k >= 0; k-- {
			switch masked[k] {
			case '}':
				depth++
			case '{':
				if depth == 0 {
					prefix := masked[:k]
					if catchOpenerRe.MatchString(prefix) {
						return true
					}
					if strings.TrimSpace(prefix) == "" {
						for p := j - 1; p >= 0; p-- {
							pmask := strings.TrimSpace(maskCommentAndStrings(lines[p].Content))
							if pmask == "" {
								continue
							}
							return catchOpenerRe.MatchString(pmask)
						}
					}
					return false
				}
				depth--
			}
		}
	}
	return false
}

func (r SLP014) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isSuppressedDebugFile(f.Path) {
			continue
		}
		isPy := strings.HasSuffix(strings.ToLower(f.Path), ".py")
		for _, h := range f.Hunks {
			for i, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					continue
				}
				stripped := stripCommentAndStrings(ln.Content)
				if stripped == "" {
					continue
				}
				matched := false
				for _, p := range debugPrintPatterns {
					if p.MatchString(stripped) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
				if isInsideCatchHandler(h.Lines, i, isPy) {
					continue
				}
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     ln.NewLineNo,
					Message:  "debug print committed — delete before merge or move to real logging",
					Snippet:  strings.TrimSpace(ln.Content),
				})
			}
		}
	}
	return out
}
