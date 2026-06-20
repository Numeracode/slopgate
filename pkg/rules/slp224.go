package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP224 flags HTTP handlers that read r.Body without checking
// ContentLength, TransferEncoding, or an explicit decode error path.
//
// Reviewer pattern:
//
//	func Handler(w http.ResponseWriter, r *http.Request) {
//	    var body Req
//	    json.NewDecoder(r.Body).Decode(&body) // ignores error, no size guard
//	}
//
// Good patterns include:
//   - r.ContentLength / r.TransferEncoding checks,
//   - explicit `if err := decoder.Decode(&body); err != nil { ... }`.
//
// Diff-based: only added lines are considered, but detection uses the whole
// handler hunk because the handler signature may be pre-existing while the
// body-reading code is added.
//
// Severity: warn (heuristic).
type SLP224 struct{}

func (SLP224) ID() string                { return "SLP224" }
func (SLP224) DefaultSeverity() Severity { return SeverityWarn }
func (SLP224) Description() string {
	return "HTTP handler reads request body without validation or decode-error handling"
}

// handlerSigRe matches Go HTTP handler signatures with `*http.Request`.
var handlerSigRe = regexp.MustCompile(`func\s+(?:\([^)]*\)\s+)?\w+\s*\([^)]*\*http\.Request(?:\s+\w+)?\s*\)`)

// bodyReadRe matches direct reads or decodes from a request body.
var bodyReadRe = regexp.MustCompile(`\b(r|req|request)\.(Body|ParseForm|ParseMultipartForm|FormValue)\b|\.Decode\s*\(&?\w+\s*\)`)

// lengthGuardRe matches ContentLength or TransferEncoding guards.
var lengthGuardRe = regexp.MustCompile(`\bContentLength\b|\bTransferEncoding\b|"chunked"|isChunked|IsChunked`)

// decodeErrGuardRe matches explicit decode error handling (`if err := ... ; err != nil`).
var decodeErrGuardRe = regexp.MustCompile(`if\s+err\s*:=\s*(?:json|xml|gob|bencode|yaml|toml|msgpack)\.NewDecoder\(`)

func (r SLP224) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Path), "_test.go") {
			continue
		}

		for _, h := range f.Hunks {
			var lines []diff.Line
			for _, ln := range h.Lines {
				if ln.Kind == diff.LineAdd {
					lines = append(lines, ln)
				}
			}
			if len(lines) == 0 {
				continue
			}

			// Build a window that includes added lines plus any nearby unchanged
			// lines so that we can see the handler signature even if it is not
			// part of the hunk.
			joined := ""
			for _, ln := range h.Lines {
				joined += ln.Content + "\n"
			}

			if !handlerSigRe.MatchString(joined) || !bodyReadRe.MatchString(joined) {
				continue
			}

			// Emit a finding when the first body read is added in a handler
			// hunk without guards or explicit decode error handling.
			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd || !bodyReadRe.MatchString(ln.Content) {
					continue
				}
				hunkText := ""
				for _, l := range h.Lines {
					if l.Kind == diff.LineAdd || l.Kind == diff.LineContext {
						hunkText += l.Content + "\n"
					}
				}
				if lengthGuardRe.MatchString(hunkText) || decodeErrGuardRe.MatchString(hunkText) {
					continue
				}
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     ln.NewLineNo,
					Message:  "HTTP handler reads request body without ContentLength/TransferEncoding check or decode-error handling",
					Snippet:  strings.TrimSpace(ln.Content),
				})
				break // one finding per hunk is enough
			}
		}
	}
	return out
}
