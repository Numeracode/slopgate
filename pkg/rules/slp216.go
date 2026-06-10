package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP216 flags shallow error logging: catch(err) blocks that log only
// err.message or err.code instead of the full error object. The stack
// trace and nested cause are lost in production logs, making incidents
// much harder to diagnose.
//
// Reviewer pattern (whimsy PR #1968, reviewer: CodeRabbit/Qodo):
// the bad form writes logger.error('msg:', err.message), which discards
// the stack; the good form passes the err object directly or wraps it
// in logger.error({err}, 'msg') so both stack and cause survive.
//
// Heuristic:
//   - JS/TS file (non-test)
//   - Added line contains a logging call (console.*|logger.*|log|slog|pino|winston|bunyan|console)
//   - AND that line references err.message, err.code, err.name, error.message
//     — but NOT the err object itself in a spread or as a second arg.
type SLP216 struct{}

func (SLP216) ID() string                { return "SLP216" }
func (SLP216) DefaultSeverity() Severity { return SeverityWarn }
func (SLP216) Description() string {
	return "error logging uses err.message — full error object preserves stack and cause"
}

// shallowErrorPropRe matches property-only accesses on an error variable:
// err.message, err.code, err.name, error.message, e.message.
var shallowErrorPropRe = regexp.MustCompile(`\b(err|error|e|ex)\.(message|code|name)\b`)

// loggerCallRe matches a JS/TS log call site.
var loggerCallRe = regexp.MustCompile(`\b(console\.(log|warn|error|info|debug)|logger\.(log|warn|error|info|debug)|log\(|slog\(|pino|winston|bunyan|console)\s*[\.(]`)

// fullErrorRe matches cases where the full error object is also passed
// (e.g. `, err)` or `, {err}` or `, { err: err}`) — i.e. log is shallow
// on one arg AND has the full object too — suppress in that case.
var fullErrorRe = regexp.MustCompile(`\b(err|error)\s*[,\)]\s*$|\{\s*(err|error)\s*[\},]|\{\s*(err|error)\s*:(err|error)\s*[\},]`)

func (r SLP216) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) {
			continue
		}
		lower := strings.ToLower(f.Path)
		if !strings.HasSuffix(lower, ".js") && !strings.HasSuffix(lower, ".ts") &&
			!strings.HasSuffix(lower, ".jsx") && !strings.HasSuffix(lower, ".tsx") &&
			!strings.HasSuffix(lower, ".mjs") && !strings.HasSuffix(lower, ".cjs") {
			continue
		}
		if strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") {
			continue
		}

		for _, ln := range f.AddedLines() {
			content := ln.Content
			// Skip if no logger call on this line.
			if !loggerCallRe.MatchString(content) {
				continue
			}
			// Skip if no shallow error prop referenced.
			if !shallowErrorPropRe.MatchString(content) {
				continue
			}
			// Skip if full error object is ALSO passed (multi-arg error log).
			// Match `, err)` or `, { err }` after stripping comments/strings.
			stripped := strings.TrimSpace(stripCommentAndStrings(content))
			if fullErrorRe.MatchString(stripped) {
				continue
			}
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     f.Path,
				Line:     ln.NewLineNo,
				Message:  "error log uses err.message/code — pass the full error object to preserve stack and cause chain",
				Snippet:  strings.TrimSpace(ln.Content),
			})
		}
	}
	return out
}
