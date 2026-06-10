package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP222 flags code that reads wmic (Windows) or subprocess output and
// runs utf8.Valid / string() on it without decoding UTF-16LE/BOM first.
// Whimsy PR #1952 snapnative.go:25 reviewer flagged "wmic outputs UTF-16LE
// with BOM by default — parsing it as UTF-8 silently loses non-ASCII."
type SLP222 struct{}

func (SLP222) ID() string                { return "SLP222" }
func (SLP222) DefaultSeverity() Severity { return SeverityWarn }
func (SLP222) Description() string {
	return "subprocess output treated as UTF-8 without BOM/UTF-16 decode check"
}

var wmicRe = regexp.MustCompile(`(?i)\bwmic( |\s+|")`)
var utf8OpRe = regexp.MustCompile(`\b(utf8\.Valid|utf8\.DecodeRune|string\s*\(\s*[^)]*out[^)]*\)|bytes\.ToString)`)
var decodeRe = regexp.MustCompile(`(?i)\b(utf16|unicode/bom|BOM|ByteOrderMark|unicode\/utf16|golang\.org\/x\/text\/encoding\/unicode)`)

func (r SLP222) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Path), "_test.go") {
			continue
		}

		for _, h := range f.Hunks {
			var hasWmic bool
			var utf8Lines []diff.Line
			var hunkText strings.Builder
			for _, ln := range h.Lines {
				if ln.Kind == diff.LineAdd {
					c := ln.Content
					hunkText.WriteString(c)
					hunkText.WriteByte('\n')
					if wmicRe.MatchString(c) {
						hasWmic = true
					}
					if utf8OpRe.MatchString(c) {
						utf8Lines = append(utf8Lines, ln)
					}
				}
			}
			if !hasWmic || len(utf8Lines) == 0 {
				continue
			}
			if decodeRe.MatchString(hunkText.String()) {
				continue
			}
			for _, ln := range utf8Lines {
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     ln.NewLineNo,
					Message:  "output treated directly as UTF-8 — wmic produces UTF-16LE with BOM by default",
					Snippet:  strings.TrimSpace(ln.Content),
				})
			}
		}
	}
	return out
}
