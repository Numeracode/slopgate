package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP159 flags subprocess-spawning helpers in test files that have no
// `timeout:` option. A stalled child process there blocks the test worker
// indefinitely, exhausting the whole CI run before the workflow-level
// timeout kicks in. Real CR-caught precedent on whimsy #1156 where one
// `connector_sdk_generation.test.js` helper had no timeout while its
// sibling `openapi_sdk_generation.test.js` already passed `timeout: 30000`.
type SLP159 struct{}

func (SLP159) ID() string                { return "SLP159" }
func (SLP159) DefaultSeverity() Severity { return SeverityWarn }
func (SLP159) Description() string {
	return "subprocess call in a test file has no timeout — a stalled child can hang the CI worker"
}

// slp159NodeCallRe matches Node child-process call sites: spawnSync, spawn,
// execSync, exec, execFileSync, execFile, fork. The opening paren must
// follow the call name (with optional whitespace) so identifiers that
// merely contain the name (e.g. `mySpawn`) are not matched.
var slp159NodeCallRe = regexp.MustCompile(`\b(spawnSync|spawn|execSync|exec|execFileSync|execFile|fork)\s*\(`)

// slp159PyCallRe matches Python subprocess call sites. The Python timeout
// keyword is `timeout=`, not `timeout:`, and we check for it below.
var slp159PyCallRe = regexp.MustCompile(`\bsubprocess\.(Popen|run|call|check_call|check_output)\s*\(`)

// slp159TimeoutJSRe matches the JS/TS options-object timeout key.
var slp159TimeoutJSRe = regexp.MustCompile(`\btimeout\s*:\s*[^,\s}]`)

// slp159TimeoutPyRe matches the Python keyword-argument timeout.
var slp159TimeoutPyRe = regexp.MustCompile(`\btimeout\s*=\s*[^,\s)]`)

// slp159SpreadOptsRe matches the JS/TS spread-an-options-variable shape
// (`...opts` / `...defaultOpts`). When present we conservatively assume the
// timeout MAY live in that spread and suppress.
var slp159SpreadOptsRe = regexp.MustCompile(`\.\.\.\w+`)

// isJSOrTSTestFile reports whether the path looks like a JS/TS unit-test
// file using the common conventions (`.test.{js,ts,jsx,tsx,mjs,cjs}`,
// `.spec.{js,ts,jsx,tsx,mjs,cjs}`, or a path under `__tests__/`).
func isJSOrTSTestFile(path string) bool {
	lower := strings.ToLower(path)
	if !isJSOrTSFile(path) {
		return false
	}
	if strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") {
		return true
	}
	if strings.Contains(lower, "/__tests__/") {
		return true
	}
	return false
}

// isPythonTestFileSlp159 mirrors the conventions in pkg/rules helpers but
// scoped to this rule's needs. It exists separately so the rule stays
// self-contained even if upstream helpers shift.
func isPythonTestFileSlp159(path string) bool {
	base := strings.ToLower(path)
	if !strings.HasSuffix(base, ".py") {
		return false
	}
	last := base
	if i := strings.LastIndex(base, "/"); i >= 0 {
		last = base[i+1:]
	}
	return strings.HasPrefix(last, "test_") || strings.HasSuffix(last, "_test.py")
}

// hasTimeoutNearby walks forward up to lookahead lines from idx (inclusive)
// and reports whether any contains a `timeout` option/keyword pattern or a
// spread of an options variable that might carry one. The forward window
// covers the multi-line option-object style:
//
//	spawnSync(cmd, args, {
//	  cwd: repoRoot,
//	  encoding: 'utf8',
//	  timeout: 30000,
//	})
func hasTimeoutNearby(lines []diff.Line, idx int, lookahead int, python bool) bool {
	end := idx + lookahead
	if end > len(lines) {
		end = len(lines)
	}
	for j := idx; j < end; j++ {
		c := lines[j].Content
		if python {
			if slp159TimeoutPyRe.MatchString(c) {
				return true
			}
			continue
		}
		if slp159TimeoutJSRe.MatchString(c) || slp159SpreadOptsRe.MatchString(c) {
			return true
		}
	}
	return false
}

func (r SLP159) Check(d *diff.Diff) []Finding {
	if d == nil {
		return nil
	}
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		js := isJSOrTSTestFile(f.Path)
		py := isPythonTestFileSlp159(f.Path)
		if !js && !py {
			continue
		}
		for _, h := range f.Hunks {
			for i, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					continue
				}
				stripped := stripCommentAndStrings(ln.Content)
				if stripped == "" {
					continue
				}
				var matched bool
				if js {
					matched = slp159NodeCallRe.MatchString(stripped)
				} else {
					matched = slp159PyCallRe.MatchString(stripped)
				}
				if !matched {
					continue
				}
				// Lookahead window of 5 lines covers the common multi-line
				// options-object shape. Same-line timeout also fires through
				// the same path (lookahead starts at idx).
				if hasTimeoutNearby(h.Lines, i, 5, py) {
					continue
				}
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     ln.NewLineNo,
					Message:  "subprocess call has no timeout — a stalled child can hang the test worker",
					Snippet:  strings.TrimSpace(ln.Content),
				})
			}
		}
	}
	return out
}
