package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP223 flags ignored error returns that are commonly introduced in new
// code and can hide failures. It looks for:
//
//   _ = f.Close()
//   _ = os.Mkdir(path, perm)
//   _ = rows.Close()
//   _ = ioutil.WriteFile(...)
//   _ = w.Write(...)
//   _ = someMethodReturningError(...)
//
// within added diff hunks. Common safe patterns are excluded, such as
// `_ = json.NewEncoder(w).Encode(v)` when used inside a deferred close
// helper or inside a deferred function.
//
// The check is diff-based: only `+` lines are scanned.
//
// Severity: warn (heuristic).
type SLP223 struct{}

func (SLP223) ID() string                { return "SLP223" }
func (SLP223) DefaultSeverity() Severity { return SeverityWarn }
func (SLP223) Description() string {
	return "returned error is explicitly ignored"
}

// ignoredErrRe matches `_ = <call>(...)`.
var ignoredErrRe = regexp.MustCompile(`(?m)^\s*(_)\s*=\s*([A-Za-z][A-Za-z0-9_\.]*)\s*\(`)

// safeCallRe matches well-known calls where ignoring the result is safe and
// idiomatic. Keep this conservative and additive.
var safeCallRe = regexp.MustCompile(`\b(json|yaml)\.NewEncoder\([^)]*\)\.Encode\(`)

// deferredFuncStartRe detects the start of a deferred anonymous function
// (`defer func() {`).
var deferredFuncStartRe = regexp.MustCompile(`\bdefer\s+func\s*\(\s*\)\s*\{`)

// callName returns the last identifier segment of an ignored call.
func callName(expr string) string {
	i := strings.LastIndex(expr, ".")
	if i == -1 {
		return expr
	}
	return expr[i+1:]
}

// inDeferredFunc returns true if the current line appears inside a `defer
// func() { ... }()` closure within the added hunk. It scans the added lines
// from the beginning up to idx.
func inDeferredFunc(hunk []diff.Line, idx int) bool {
	// Walk backwards from the line within the hunk to find the `defer func() {`
	// that contains it. We use simple brace counting, considering both added
	// and context lines because the closure start may not be added in this hunk.
	depth := 0
	for i := idx; i >= 0; i-- {
		ln := hunk[i]
		content := ln.Content
		openBraces := strings.Count(content, "{")
		closeBraces := strings.Count(content, "}")

		if i == idx {
			openBraces = 0
		}
		if deferredFuncStartRe.MatchString(content) {
			openBraces--
			depth += openBraces - closeBraces
			return depth >= 0
		}
		depth += openBraces - closeBraces
	}
	return false
}

func (r SLP223) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Path), "_test.go") {
			continue
		}
		for _, h := range f.Hunks {
			var added []diff.Line
			var addedIdx []int
			for i, ln := range h.Lines {
				if ln.Kind == diff.LineAdd {
					added = append(added, ln)
					addedIdx = append(addedIdx, i)
				}
			}
			for idx, ln := range added {
				content := ln.Content
				m := ignoredErrRe.FindStringSubmatch(content)
				if m == nil {
					continue
				}
				call := m[2]
				if safeCallRe.MatchString(content) && inDeferredFunc(h.Lines, addedIdx[idx]) {
					continue
				}
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     ln.NewLineNo,
					Message:  "ignored error return: " + callName(call) + "() error is discarded",
					Snippet:  strings.TrimSpace(content),
				})
			}
		}
	}
	return out
}
