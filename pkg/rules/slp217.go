package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP217 flags newly-added functions whose path-like string parameters
// (e.g. sourceRoot, destPath, stagingDir) are not validated for empty or
// whitespace-only input within the hunk that introduces them.
//
// Reviewer pattern (whimsy PR #1962 backup.go:61, PR #1961 push.go:113):
//
//	func Backup(sourceRoot, remoteDest string) (*Result, error) {
//	    // no empty-check on either param
//	    stagingDir := ...
//	}
//
// Heuristic (diff-level, language-agnostic for Go + JS/TS):
//   - Added function definition introduces a path-shaped parameter
//     (param name ends in Path/Dir/Dest/Root/Remote as camelCase or
//     snake_case).
//   - The same hunk does NOT contain an early validation expression:
//     len(X) == 0, X == "", strings.TrimSpace(X), !X (JS truthy),
//     if (!X), require(X), assert(X), or similar.
//   - Severity: warn — this is a heuristic, so not a block.
type SLP217 struct{}

func (SLP217) ID() string                { return "SLP217" }
func (SLP217) DefaultSeverity() Severity { return SeverityWarn }
func (SLP217) Description() string {
	return "path-like parameter not validated for empty input"
}

// goFuncParamRe captures Go function definitions with their parameter list.
// Matches: func Foo(sourceRoot, remoteDest string) and
//          func (r *Runner) Do(destDir string) { ... }
var goFuncParamRe = regexp.MustCompile(`(?m)^\s*func\s+(?:\([^)]+\)\s+)?\w+\s*\(([^)]*)\)`)

// jsFuncParamRe captures JS/TS named function or arrow function definitions
// with their parameter list. Matches `function foo(a, b)` and `const foo = (a, b) =>`.
var jsFuncParamRe = regexp.MustCompile(`(?m)\bfunction\s+\w+\s*\(([^)]*)\)|\(\s*([^)]*?)\s*\)\s*(?::\s*[^=]+)?\s*=>`)

// pathShapedRe identifies parameter names that look like file-system paths.
// Supports camelCase (destPath, stagingDir) and snake_case (src_root, staging_dir).
var pathShapedRe = regexp.MustCompile(`\b[\w]+(Path|Dir|Dest|Root|Remote|Source)\b`)

// validationSignalRe matches code patterns that indicate the parameter is
// being validated for empty/whitespace/truthy input.
var validationSignalRe = regexp.MustCompile(`len\(\s*` + pathParamPlaceholder + `\s*\)\s*==\s*0|` +
	`\b` + pathParamPlaceholder + `\s*==\s*""|` +
	`strings\.TrimSpace\s*\(\s*` + pathParamPlaceholder + `\s*\)|` +
	`\brequire\(\s*` + pathParamPlaceholder + `\s*[\),]|` +
	`\brequireNonEmpty|ioutil\.ReadDir\(\s*` + pathParamPlaceholder)

const pathParamPlaceholder = `PARAMNAME`

func isGoFilePath(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".go") &&
		!strings.HasSuffix(strings.ToLower(path), "_test.go")
}

func isJSFilePath(path string) bool {
	l := strings.ToLower(path)
	return strings.HasSuffix(l, ".js") || strings.HasSuffix(l, ".ts") ||
		strings.HasSuffix(l, ".jsx") || strings.HasSuffix(l, ".tsx") ||
		strings.HasSuffix(l, ".mjs") || strings.HasSuffix(l, ".cjs")
}

// extractPathShapedParams returns the parameter names that look like
// filesystem paths.
func extractPathShapedParams(paramBlock string) []string {
	// Strip comments and default values, then split on comma.
	// We only need the name portion of each param.
	var out []string
	// Remove string literal default values that might contain "Path" or "Dir".
	scrubbed := strings.ReplaceAll(paramBlock, "`", "")
	for _, part := range strings.Split(scrubbed, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Go: `sourceRoot string` or `remoteDest string`
		// JS: `sourceRoot` or `sourceRoot: string`
		name := strings.Fields(part)[0]
		name = strings.TrimRight(name, "?:")
		if !pathShapedRe.MatchString(name) {
			continue
		}
		out = append(out, name)
	}
	return out
}

func hunkValidatesParam(hunkLines []diff.Line, param string) bool {
	// Build one string of all added lines in the hunk with the param name
	// available for pattern-matching.
	for _, ln := range hunkLines {
		if ln.Kind != diff.LineAdd {
			continue
		}
		content := ln.Content
		// Check several direct patterns — faster than building a full regex.
		if strings.Contains(content, "len("+param+")") ||
			strings.Contains(content, param+` == ""`) ||
			strings.Contains(content, `"" == `+param) ||
			strings.Contains(content, param+" ==") ||
			strings.Contains(content, "strings.TrimSpace("+param+")") ||
			strings.Contains(content, "requireNonEmpty("+param) ||
			strings.Contains(content, "requireNonblank("+param) ||
			strings.Contains(content, "requireParam(") {
			return true
		}
		// JS truthy: `if (!sourceRoot)` or `if(!sourceRoot)`
		if strings.Contains(content, "!"+param) {
			return true
		}
	}
	return false
}

func (r SLP217) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) {
			continue
		}
		isGo := isGoFilePath(f.Path)
		isJS := isJSFilePath(f.Path)
		if !isGo && !isJS {
			continue
		}

		for _, h := range f.Hunks {
			// Concatenate all added lines in the hunk for function-def matching.
			var added []diff.Line
			for _, ln := range h.Lines {
				if ln.Kind == diff.LineAdd {
					added = append(added, ln)
				}
			}
			if len(added) == 0 {
				continue
			}
			joined := ""
			for _, ln := range added {
				joined += ln.Content + "\n"
			}

			// Extract parameter blocks.
			var paramBlocks []string
			if isGo {
				for _, m := range goFuncParamRe.FindAllStringSubmatch(joined, -1) {
					if len(m) >= 2 {
						paramBlocks = append(paramBlocks, m[1])
					}
				}
			}
			if isJS {
				for _, m := range jsFuncParamRe.FindAllStringSubmatch(joined, -1) {
					if len(m) >= 2 && m[1] != "" {
						paramBlocks = append(paramBlocks, m[1])
					}
					if len(m) >= 3 && m[2] != "" {
						paramBlocks = append(paramBlocks, m[2])
					}
				}
			}

			for _, pb := range paramBlocks {
				params := extractPathShapedParams(pb)
				for _, p := range params {
					if !hunkValidatesParam(h.Lines, p) {
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     added[0].NewLineNo,
							Message:  "parameter \"" + p + "\" (path/dir/dest/root) has no empty-input check in this hunk",
							Snippet:  strings.TrimSpace(added[0].Content),
						})
					}
				}
			}
		}
	}
	return out
}
