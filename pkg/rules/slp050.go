//go:build ignore_retired

package rules

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP050 flags Go functions that accept pointer, slice, map, interface, or
// string parameters without performing any nil/empty validation.
//
// Rationale: Missing validation leads to runtime panics. If a function
// receives a `*T` or `[]T` and never checks it before use, it will crash on
// nil input. AI-generated code often omits these guards.
type SLP050 struct{}

func (SLP050) ID() string                { return "SLP050" }
func (SLP050) DefaultSeverity() Severity { return SeverityWarn }
func (SLP050) Description() string {
	return "function accepts pointer or reference param without nil/empty validation"
}

var (
	// slp050FuncDeclRe captures the function name and opening parameter paren.
	slp050FuncDeclRe = regexp.MustCompile(`^\s*func\s+(?:\([^)]+\)\s*)?([A-Za-z_]\w*)(?:\s*\[[^\]]+\])?\s*\(`)
)

func (r SLP050) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !isGoFile(f.Path) {
			continue
		}
		for _, h := range f.Hunks {
			lines := h.Lines
			for i := 0; i < len(lines); i++ {
				ln := lines[i]
				if ln.Kind != diff.LineAdd {
					continue
				}
				match := slp050FuncDeclRe.FindStringSubmatchIndex(ln.Content)
				if match == nil {
					continue
				}

				funcName := ln.Content[match[2]:match[3]]
				header, _, ok := slp050CollectFuncHeader(lines, i, match[1]-1)
				if !ok {
					continue
				}

				paramNames := slp050ValidatedParams(header)
				if len(paramNames) == 0 {
					continue
				}

				paramChecks := make(map[string]*slp050ParamChecks, len(paramNames))
				for _, p := range paramNames {
					paramChecks[p] = newSLP050ParamChecks(p)
				}

				validated := make(map[string]bool)
				depth := 0
				bodyStarted := false
				for j := i; j < len(lines); j++ {
					next := lines[j]
					clean := stripCommentAndStrings(next.Content)
					if !bodyStarted && strings.Contains(clean, "{") {
						bodyStarted = true
					}
					if bodyStarted && next.Kind == diff.LineAdd && clean != "" {
						for _, p := range paramNames {
							if paramChecks[p].matches(clean) {
								validated[p] = true
							}
						}
					}
					depth += slp050BraceDelta(next.Content)
					if bodyStarted && depth <= 0 {
						break
					}
				}

				for _, p := range paramNames {
					if !validated[p] {
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     ln.NewLineNo,
							Message:  "function " + funcName + " accepts " + p + " without validation — add nil/empty check",
							Snippet:  strings.TrimSpace(ln.Content),
						})
					}
				}
			}
		}
	}
	return out
}

type slp050ParamChecks struct {
	nilCheck   *regexp.Regexp
	emptyCheck *regexp.Regexp
	lenCheck   *regexp.Regexp
}

func newSLP050ParamChecks(name string) *slp050ParamChecks {
	ident := regexp.QuoteMeta(name)
	return &slp050ParamChecks{
		nilCheck: regexp.MustCompile(`\b` + ident + `\b\s*(?:==|!=)\s*nil|\bnil\s*(?:==|!=)\s*\b` + ident + `\b`),
		emptyCheck: regexp.MustCompile(
			`\b` + ident + `\b\s*(?:==|!=)\s*""|""\s*(?:==|!=)\s*\b` + ident + `\b`,
		),
		lenCheck: regexp.MustCompile(
			`len\s*\(\s*` + ident + `\s*\)\s*(?:==|!=|>=|<=)\s*0|len\s*\(\s*` + ident + `\s*\)\s*>\s*0|len\s*\(\s*` + ident + `\s*\)\s*<\s*1`,
		),
	}
}

func (c *slp050ParamChecks) matches(line string) bool {
	return c.nilCheck.MatchString(line) || c.emptyCheck.MatchString(line) || c.lenCheck.MatchString(line)
}

func slp050BraceDelta(line string) int {
	clean := stripCommentAndStrings(line)
	return strings.Count(clean, "{") - strings.Count(clean, "}")
}

func slp050CollectFuncHeader(lines []diff.Line, start, openParen int) (string, int, bool) {
	var b strings.Builder
	depth := 1

	for i := start; i < len(lines); i++ {
		content := lines[i].Content
		scanStart := 0
		if i == start {
			scanStart = openParen + 1
		}
		for j := scanStart; j < len(content); j++ {
			switch content[j] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					if i > start {
						b.WriteByte('\n')
					}
					b.WriteString(content[:j+1])
					return b.String(), i, true
				}
			}
		}

		if i > start {
			b.WriteByte('\n')
		}
		b.WriteString(content)
	}

	return "", 0, false
}

func slp050ValidatedParams(header string) []string {
	src := "package p\n" + header + " {}\n"
	file, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
	if err != nil || len(file.Decls) == 0 {
		return nil
	}

	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Type == nil || fn.Type.Params == nil {
		return nil
	}

	var out []string
	for _, field := range fn.Type.Params.List {
		if !slp050NeedsValidation(field.Type) {
			continue
		}
		for _, name := range field.Names {
			out = append(out, name.Name)
		}
	}
	return out
}

func slp050NeedsValidation(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return true
	case *ast.ArrayType:
		return t.Len == nil
	case *ast.MapType:
		return true
	case *ast.InterfaceType:
		return len(t.Methods.List) == 0
	case *ast.Ident:
		return t.Name == "any" || t.Name == "string"
	default:
		return false
	}
}
