package rules

import (
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP109 flags two or more functions added in the same file with highly
// similar bodies (>60% identical). This is a common AI slop pattern:
// copy-pasting entire functions with minor changes instead of extracting
// shared logic.
type SLP109 struct{}

func (SLP109) ID() string                { return "SLP109" }
func (SLP109) DefaultSeverity() Severity { return SeverityWarn }
func (SLP109) Description() string {
	return "duplicate function body — extract shared logic instead of copy-pasting"
}

func slp109Normalize(line string) string {
	s := strings.TrimSpace(line)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + 32)
		}
	}
	return b.String()
}

func slp109BodySimilarity(a, b []string) float64 {
	setA := make(map[string]bool)
	for _, la := range a {
		na := slp109Normalize(la)
		if len(na) >= 3 {
			setA[na] = true
		}
	}
	setB := make(map[string]bool)
	for _, lb := range b {
		nb := slp109Normalize(lb)
		if len(nb) >= 3 {
			setB[nb] = true
		}
	}
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	intersection := 0
	for k := range setA {
		if setB[k] {
			intersection++
		}
	}
	maxLen := len(setA)
	if len(setB) > maxLen {
		maxLen = len(setB)
	}
	return float64(intersection) / float64(maxLen)
}

func slp109HasCallableShape(content string) bool {
	return strings.Contains(content, "(")
}

func slp109HasFuncKeyword(content string) bool {
	cLower := strings.ToLower(content)
	return strings.Contains(cLower, "func ") ||
		strings.Contains(cLower, "function ") ||
		strings.Contains(cLower, "public ") ||
		strings.Contains(cLower, "private ") ||
		strings.Contains(cLower, "protected ") ||
		strings.Contains(cLower, "static ")
}

func slp109CollectSignature(lines []diff.Line, start int) (string, int, int, bool) {
	var parts []string
	parenDepth := 0
	for i := start; i < len(lines); i++ {
		if lines[i].Kind != diff.LineAdd {
			return "", 0, 0, false
		}
		raw := lines[i].Content
		content := strings.TrimSpace(raw)
		if content == "" {
			continue
		}
		parts = append(parts, content)
		// Scan raw line to track paren nesting and find the true body-opening '{'.
		bodyBraceOff := -1
		depth := parenDepth
		for off, ch := range raw {
			switch ch {
			case '(':
				depth++
			case ')':
				if depth > 0 {
					depth--
				}
			case '[':
				depth++
			case ']':
				if depth > 0 {
					depth--
				}
			case '{':
				if depth == 0 && bodyBraceOff < 0 {
					bodyBraceOff = off
				}
			}
		}
		parenDepth = depth
		if bodyBraceOff >= 0 {
			return strings.Join(parts, " "), i, bodyBraceOff, true
		}
		if strings.Contains(content, ";") {
			return "", 0, 0, false
		}
	}
	return "", 0, 0, false
}

func (r SLP109) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) {
			continue
		}
		if !isGoFile(f.Path) && !isJSOrTSFile(f.Path) && !isJavaFile(f.Path) {
			continue
		}

		type funcBody struct {
			sigLine       int
			bodyStartLine int
			braceOff      int
			sig           string
			body          []string
		}
		var funcs []funcBody

		for _, h := range f.Hunks {
			inFunc := false
			braceDepth := 0
			var cur funcBody

			for idx := 0; idx < len(h.Lines); idx++ {
				ln := h.Lines[idx]
				if !inFunc {
					if ln.Kind != diff.LineAdd {
						continue
					}
					content := strings.TrimSpace(ln.Content)
					if !slp109HasFuncKeyword(content) {
						continue
					}
					if !slp109HasCallableShape(content) {
						continue
					}
					sig, bodyStart, braceOff, ok := slp109CollectSignature(h.Lines, idx)
					if !ok {
						continue
					}
					inFunc = true
					braceDepth = 0
					cur = funcBody{sigLine: ln.NewLineNo, bodyStartLine: h.Lines[bodyStart].NewLineNo, braceOff: braceOff, sig: sig}
					if bodyStart == idx {
						lineContent := h.Lines[bodyStart].Content
						if braceOff >= 0 && braceOff < len(lineContent) {
							closeIdx := strings.LastIndex(lineContent, "}")
							if closeIdx > braceOff+1 {
								inner := strings.TrimSpace(lineContent[braceOff+1 : closeIdx])
								if inner != "" {
									cur.body = append(cur.body, inner)
								}
							} else if braceOff+1 < len(lineContent) {
								// No closing brace on same line — capture content after the opening brace.
								inner := strings.TrimSpace(lineContent[braceOff+1:])
								if inner != "" {
									cur.body = append(cur.body, inner)
								}
							}
						}
					}
					idx = bodyStart - 1
					continue
				}

				if ln.Kind == diff.LineDelete {
					inFunc = false
					cur = funcBody{}
					continue
				}
				if ln.Kind != diff.LineAdd {
					inFunc = false
					cur = funcBody{}
					continue
				}
				content := strings.TrimSpace(ln.Content)
				sanitized := stripCommentAndStrings(content)
				braceDepth += strings.Count(sanitized, "{")
				braceDepth -= strings.Count(sanitized, "}")
				if ln.Kind == diff.LineAdd && ln.NewLineNo != cur.sigLine {
					if ln.NewLineNo == cur.bodyStartLine {
						// For multi-line signatures: capture content after the opening '{' on the body-start line.
						// braceOff is an offset into the raw line, so the
						// slice must use ln.Content, not the trimmed copy —
						// indexing the shorter trimmed string panics.
						bracePos := cur.braceOff
						if bracePos >= 0 && bracePos < len(ln.Content) {
							inner := strings.TrimSpace(ln.Content[bracePos+1:])
							inner = strings.TrimRight(inner, "} 	")
							if inner != "" {
								cur.body = append(cur.body, inner)
							}
						}
					} else if content != "{" && content != "}" {
						cur.body = append(cur.body, content)
					}
				}
				if braceDepth <= 0 && strings.Contains(sanitized, "}") {
					if len(cur.body) > 0 {
						funcs = append(funcs, cur)
					}
					inFunc = false
				}
			}
		}

		seen := make(map[int]bool)
		for i := 0; i < len(funcs); i++ {
			for j := i + 1; j < len(funcs); j++ {
				if seen[funcs[j].sigLine] {
					continue
				}
				sim := slp109BodySimilarity(funcs[i].body, funcs[j].body)
				if sim > 0.6 {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     funcs[j].sigLine,
						Message:  "function body is highly similar to another added function — extract shared logic",
						Snippet:  funcs[j].sig,
					})
					seen[funcs[j].sigLine] = true
				}
			}
		}
	}
	return out
}
