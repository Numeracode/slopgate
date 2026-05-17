package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP152 flags unreachable code that follows an if/else chain in which
// every branch — including a terminal else — ends with a control-flow
// terminator. SLP019 already flags code after a single terminator;
// SLP152 extends that to a fully-terminating conditional, a common
// artifact of an AI agent appending code after an if/else that always
// returns.
//
// Restricted to brace languages: there, a terminator nested inside a
// branch is always followed by a closing brace, so a branch's last
// line is a terminator only when the branch unconditionally exits.
type SLP152 struct{}

func (SLP152) ID() string                { return "SLP152" }
func (SLP152) DefaultSeverity() Severity { return SeverityWarn }
func (SLP152) Description() string {
	return "unreachable code after an if/else chain whose every branch terminates"
}

var (
	slp152IfOpen   = regexp.MustCompile(`^if\b.*\{$`)
	slp152ElseIf   = regexp.MustCompile(`^\}\s*else\s+if\b.*\{$`)
	slp152Else     = regexp.MustCompile(`^\}\s*else\s*\{$`)
	slp152TermWord = regexp.MustCompile(`^(?:return|throw|panic|break|continue)\b`)
	slp152TermCall = regexp.MustCompile(`\b(?:os\.Exit|process\.exit|log\.(?:Fatal|Panic)[A-Za-z]*)\s*\(`)
)

// slp152Chain accumulates, for one if/else chain, whether every branch
// closed so far ended with a terminator and whether a terminal else
// has been seen.
type slp152Chain struct {
	allTerminate bool
	hasElse      bool
}

func slp152Indent(raw string) int {
	n := 0
	for _, c := range raw {
		if c != ' ' && c != '\t' {
			break
		}
		n++
	}
	return n
}

// slp152IsTerminator reports whether a stripped line is a statement
// that unconditionally ends its branch.
func slp152IsTerminator(stripped string) bool {
	return slp152TermWord.MatchString(stripped) || slp152TermCall.MatchString(stripped)
}

// slp152DeadLineAfter returns the index of the first real statement at
// the chain's indentation following the chain's closing brace, or -1
// when the next line is structural or at another indentation.
func slp152DeadLineAfter(lines []diff.Line, closeIdx, indent int) int {
	for j := closeIdx + 1; j < len(lines); j++ {
		s := strings.TrimSpace(stripCommentAndStrings(lines[j].Content))
		if s == "" {
			continue
		}
		if slp152Indent(lines[j].Content) != indent {
			return -1
		}
		if s == "}" || slp152Else.MatchString(s) || slp152ElseIf.MatchString(s) {
			return -1
		}
		return j
	}
	return -1
}

func slp152CheckHunk(r SLP152, path string, lines []diff.Line) []Finding {
	var out []Finding
	chains := map[int]*slp152Chain{}
	prevStripped := ""
	prevIndent := -1

	for i, ln := range lines {
		indent := slp152Indent(ln.Content)
		stripped := strings.TrimSpace(stripCommentAndStrings(ln.Content))
		if stripped == "" {
			continue
		}

		switch {
		case slp152IfOpen.MatchString(stripped):
			chains[indent] = &slp152Chain{allTerminate: true}
		case slp152ElseIf.MatchString(stripped):
			if c := chains[indent]; c != nil {
				c.allTerminate = c.allTerminate && prevIndent > indent && slp152IsTerminator(prevStripped)
			}
		case slp152Else.MatchString(stripped):
			if c := chains[indent]; c != nil {
				c.allTerminate = c.allTerminate && prevIndent > indent && slp152IsTerminator(prevStripped)
				c.hasElse = true
			}
		case stripped == "}":
			if c := chains[indent]; c != nil {
				c.allTerminate = c.allTerminate && prevIndent > indent && slp152IsTerminator(prevStripped)
				delete(chains, indent)
				if c.hasElse && c.allTerminate {
					if dead := slp152DeadLineAfter(lines, i, indent); dead >= 0 {
						if d := lines[dead]; d.Kind == diff.LineAdd {
							out = append(out, Finding{
								RuleID:   r.ID(),
								Severity: r.DefaultSeverity(),
								File:     path,
								Line:     d.NewLineNo,
								Message:  "unreachable code: the preceding if/else chain terminates in every branch",
								Snippet:  strings.TrimSpace(d.Content),
							})
						}
					}
				}
			}
		}

		prevStripped = stripped
		prevIndent = indent
	}
	return out
}

func (r SLP152) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) {
			continue
		}
		if !isGoFile(f.Path) && !isJSOrTSFile(f.Path) && !isJavaFile(f.Path) && !isRustFile(f.Path) {
			continue
		}
		for _, h := range f.Hunks {
			out = append(out, slp152CheckHunk(r, f.Path, h.Lines)...)
		}
	}
	return out
}
