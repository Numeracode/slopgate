package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP218 flags Go HTTP handlers that gate body-reading on
// `r.ContentLength > 0` (or <= 0, == -1, etc.) without also considering
// chunked transfer-encoding. The chunked case has ContentLength == -1,
// so a bare `ContentLength > 0` check drops chunked requests silently.
//
// Reviewer pattern (whimsy PR #1967 handlers_e2ee.go:165):
//
//	if r.ContentLength > 0 {
//	    _ = json.NewDecoder(r.Body).Decode(&body)
//	}
//
// Correct pattern also checks r.TransferEncoding or reads regardless.
type SLP218 struct{}

func (SLP218) ID() string                { return "SLP218" }
func (SLP218) DefaultSeverity() Severity { return SeverityBlock }
func (SLP218) Description() string {
	return "ContentLength>0 check without Transfer-Encoding handling drops chunked requests"
}

// contentLengthGateRe matches ContentLength checks used as body gates.
var contentLengthGateRe = regexp.MustCompile(`\b(r|req|request)\.ContentLength\s*(>|<=|==|!=)\s*-?\d+`)

// transferEncodingRef matches code that acknowledges chunked encoding.
var transferEncodingRef = regexp.MustCompile(`(?i)\bTransferEncoding|transfer[-_]encoding|"chunked"|isChunked|IsChunked`)

func (r SLP218) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Path), "_test.go") {
			continue
		}

		for _, h := range f.Hunks {
			// Build one string of all added content in the hunk.
			var addedText string
			var addedLines []diff.Line
			for _, ln := range h.Lines {
				if ln.Kind == diff.LineAdd {
					addedText += ln.Content + "\n"
					addedLines = append(addedLines, ln)
				}
			}
			if len(addedLines) == 0 {
				continue
			}

			// If the hunk already considers TransferEncoding, don't flag.
			if transferEncodingRef.MatchString(addedText) {
				continue
			}

			// Flag each added line that carries a bare ContentLength gate.
			for _, ln := range addedLines {
				if !contentLengthGateRe.MatchString(ln.Content) {
					continue
				}
				// Skip if the line also uses ContentLength <= 0 as a
				// fall-through "body might be here" guard — that's actually
				// the *correct* idiom when paired with an unconditional
				// decode in an else branch or later in the function.
				// Skip non-gating comparisons: <= 0, == -1, and == 0
				// are all "there's no body" checks, not body-present gates.
				if strings.Contains(ln.Content, "<= 0") || strings.Contains(ln.Content, "== -1") || strings.Contains(ln.Content, "== 0") {
					continue
				}
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     ln.NewLineNo,
					Message:  "ContentLength gate misses chunked transfer — also check r.TransferEncoding or read body unconditionally",
					Snippet:  strings.TrimSpace(ln.Content),
				})
			}
		}
	}
	return out
}
