package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP218 flags two related HTTP request-body handling smells:
//
// 1. Go HTTP handlers that gate body-reading on `r.ContentLength > 0`
//    (or <= 0, == -1, etc.) without also considering chunked
//    transfer-encoding. The chunked case has ContentLength == -1, so a
//    bare `ContentLength > 0` check drops chunked requests silently.
//
//    Reviewer pattern (whimsy PR #1967 handlers_e2ee.go:165):
//
//	  if r.ContentLength > 0 {
//	      _ = json.NewDecoder(r.Body).Decode(&body)
//	  }
//
//    Correct pattern also checks r.TransferEncoding or reads regardless.
//
// 2. Constructing a url.URL with Scheme "file" and putting path-like data
//    in Opaque while Path remains unset. The Opaque field is meant for
//    non-hierarchical URIs; for file URLs the path should be assigned
//    (and filepath.ToSlash applied when converting OS paths).
//
//    Reviewer pattern: url.URL{Scheme:"file", Opaque: p} where p looks
//    like a filesystem path. Correct: url.URL{Scheme:"file", Path:
//    filepath.ToSlash(p)}.
type SLP218 struct{}

func (SLP218) ID() string                { return "SLP218" }
func (SLP218) DefaultSeverity() Severity { return SeverityBlock }
func (SLP218) Description() string {
	return "HTTP body/URL handling ignores chunked transfer or misuses file URL Opaque"
}

// contentLengthGateRe matches ContentLength checks used as body gates.
var contentLengthGateRe = regexp.MustCompile(`\b(r|req|request)\.ContentLength\s*(>|<=|==|!=)\s*-?\d+`)

// nonGatingRe matches ContentLength comparisons that are non-gating (inequality
// against 0 or -1) — these exclude a zero-length body but don't help with
// chunked encoding where ContentLength == -1.
var nonGatingRe = regexp.MustCompile(`\b(r|req|request)\.ContentLength\s*(<= 0|== -1|== 0)\b`)

// transferEncodingRef matches code that acknowledges chunked encoding.
var transferEncodingRef = regexp.MustCompile(`(?i)\bTransferEncoding|transfer[-_]encoding|"chunked"|isChunked|IsChunked`)

// fileOpaqueRe matches url.URL{Scheme:"file", Opaque: ...} constructions.
// Group 1 is the variable holding path-like data assigned to Opaque.
var fileOpaqueRe = regexp.MustCompile(`url\.URL\s*\{\s*Scheme\s*:\s*"file"\s*,\s*Opaque\s*:\s*([^,}\s]+)`)

// pathLikeOpaqueRe matches Opaque values that look like filesystem paths.
// A variable named path is enough to treat as path-like (per reviewer pattern).
var pathLikeOpaqueRe = regexp.MustCompile(`[/.\\~]|^\w*[Pp]ath(\w*)$`)

func (r SLP218) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Path), "_test.go") {
			continue
		}
		isGo := true

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

			// 1. ContentLength/TransferEncoding check.
			if !transferEncodingRef.MatchString(addedText) {
				for _, ln := range addedLines {
					if !contentLengthGateRe.MatchString(ln.Content) {
						continue
					}
					// Skip non-gating comparisons: <= 0, == -1, and == 0
					// are all "there's no body" checks, not body-present gates.
					if nonGatingRe.MatchString(ln.Content) {
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

			// 2. File-URL Opaque misuse (Go only).
			if isGo {
				for _, m := range fileOpaqueRe.FindAllStringSubmatch(addedText, -1) {
					if len(m) < 2 {
						continue
					}
					opaque := strings.TrimSpace(m[1])
					if !pathLikeOpaqueRe.MatchString(opaque) {
						continue
					}
					// Determine the line number of the Opaque assignment.
					var lineNo int
					for _, ln := range addedLines {
						if strings.Contains(ln.Content, "Opaque:") {
							lineNo = ln.NewLineNo
							break
						}
					}
					if lineNo == 0 {
						lineNo = addedLines[0].NewLineNo
					}
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     lineNo,
						Message:  "file URL assigns path-like data to Opaque instead of Path (use filepath.ToSlash)",
						Snippet:  strings.TrimSpace(m[0]),
					})
				}
			}
		}
	}
	return out
}
