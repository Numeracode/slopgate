package rules

import (
	"regexp"
	"sort"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP205 flags OpenAPI path merge order that lets generated or hardcoded
// path maps override richer JSDoc-derived spec.paths annotations.
type SLP205 struct{}

// ID returns the stable rule identifier.
func (SLP205) ID() string { return "SLP205" }

// DefaultSeverity returns the default finding severity.
func (SLP205) DefaultSeverity() Severity { return SeverityWarn }

// Description returns a short rule summary for rule catalogs.
func (SLP205) Description() string {
	return "OpenAPI path merge order lets generated paths override annotated spec paths"
}

var (
	slp205SpecPathsAssign       = regexp.MustCompile(`\bspec\.paths\s*=\s*\{`)
	slp205SpecPathsSpread       = regexp.MustCompile(`\.\.\.\s*\(?\s*spec\.paths\s*(?:\|\|\s*\{\s*\})?\s*\)?`)
	slp205GeneratedPathsSpread  = regexp.MustCompile(`(?i)\.\.\.\s*(?:oas\d+paths|openapi\w*paths|generated\w*paths|hardcoded\w*paths)\b`)
	slp205CommentOnlyLinePrefix = regexp.MustCompile(`^\s*(?://|/\*|\*|#)`)
)

type slp205Line struct {
	content   string
	kind      diff.LineKind
	newLineNo int
	order     int
}

type slp205Event struct {
	kind   string
	line   slp205Line
	pos    int
	indent int
}

// Check scans JS/TS OpenAPI assembly diffs for unsafe path map spread order.
func (r SLP205) Check(d *diff.Diff) []Finding {
	var out []Finding
	if d == nil {
		return out
	}

	for _, f := range d.Files {
		// Limit this rule to source files where OpenAPI specs are assembled.
		if f.IsDelete || !isJSOrTSFile(f.Path) || isTestFile(f.Path) ||
			isGeneratedArtifactPath(f.Path) || isOpenAPIArtifactPath(f.Path) {
			continue
		}

		for _, h := range f.Hunks {
			for i := range h.Lines {
				ln := h.Lines[i]
				if ln.Kind == diff.LineDelete || !slp205SpecPathsAssign.MatchString(ln.Content) {
					continue
				}

				block := slp205CollectObjectBlock(h.Lines, i)
				if len(block) == 0 || !slp205HasAddedRelevantLine(block) {
					continue
				}
				if generatedLine, ok := slp205BadPathMergeOrder(block); ok {
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     generatedLine.newLineNo,
						Message:  "OpenAPI generated path map is spread after spec.paths, overriding richer JSDoc annotations; spread spec.paths last",
						Snippet:  strings.TrimSpace(generatedLine.content),
					})
					break
				}
			}
		}
	}
	return out
}

func slp205CollectObjectBlock(lines []diff.Line, start int) []slp205Line {
	if len(lines) == 0 || start < 0 || start >= len(lines) {
		return nil
	}
	var out []slp205Line
	depth := 0
	started := false
	for i := start; i < len(lines); i++ {
		ln := lines[i]
		if ln.Kind == diff.LineDelete {
			continue
		}
		content := ln.Content
		if !started {
			open := strings.Index(content, "{")
			if open < 0 {
				continue
			}
			started = true
			depth += slp205BraceDelta(content[open:])
		} else {
			depth += slp205BraceDelta(content)
		}

		out = append(out, slp205Line{
			content:   content,
			kind:      ln.Kind,
			newLineNo: ln.NewLineNo,
			order:     i,
		})
		if started && depth <= 0 {
			break
		}
	}
	return out
}

func slp205BraceDelta(content string) int {
	if content == "" {
		return 0
	}
	delta := 0
	for _, r := range content {
		switch r {
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}

func slp205HasAddedRelevantLine(lines []slp205Line) bool {
	if len(lines) == 0 {
		return false
	}
	for _, ln := range lines {
		if ln.kind != diff.LineAdd || slp205CommentOnlyLinePrefix.MatchString(ln.content) {
			continue
		}
		if slp205SpecPathsAssign.MatchString(ln.content) ||
			slp205SpecPathsSpread.MatchString(ln.content) ||
			slp205GeneratedPathsSpread.MatchString(ln.content) {
			return true
		}
	}
	return false
}

func slp205BadPathMergeOrder(lines []slp205Line) (slp205Line, bool) {
	if len(lines) == 0 {
		return slp205Line{}, false
	}
	events := slp205MergeEvents(lines)
	specSeen := false
	for _, event := range events {
		switch event.kind {
		case "spec":
			if !specSeen {
				specSeen = true
			}
		case "generated":
			if specSeen {
				return event.line, true
			}
		}
	}
	return slp205Line{}, false
}

func slp205MergeEvents(lines []slp205Line) []slp205Event {
	if len(lines) == 0 {
		return nil
	}
	var candidates []slp205Event
	baseIndent := -1
	for _, ln := range lines {
		if slp205CommentOnlyLinePrefix.MatchString(ln.content) {
			continue
		}
		lineIndent := indentationOf(ln.content)
		specMatches := slp205SpecPathsSpread.FindAllStringIndex(ln.content, -1)
		for i := range specMatches {
			match := specMatches[i]
			if len(match) < 2 {
				continue
			}
			pos, ok := slp205MatchStart(match)
			if !ok {
				continue
			}
			candidates = append(candidates, slp205Event{kind: "spec", line: ln, pos: pos, indent: lineIndent})
			if baseIndent < 0 || lineIndent < baseIndent {
				baseIndent = lineIndent
			}
		}
		generatedMatches := slp205GeneratedPathsSpread.FindAllStringIndex(ln.content, -1)
		for i := range generatedMatches {
			match := generatedMatches[i]
			if len(match) < 2 {
				continue
			}
			pos, ok := slp205MatchStart(match)
			if !ok {
				continue
			}
			candidates = append(candidates, slp205Event{kind: "generated", line: ln, pos: pos, indent: lineIndent})
			if baseIndent < 0 || lineIndent < baseIndent {
				baseIndent = lineIndent
			}
		}
	}
	events := make([]slp205Event, 0, len(candidates))
	for _, event := range candidates {
		if event.indent == baseIndent {
			events = append(events, event)
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].line.order == events[j].line.order {
			return events[i].pos < events[j].pos
		}
		return events[i].line.order < events[j].line.order
	})
	return events
}

func slp205MatchStart(match []int) (int, bool) {
	// regexp.FindAllStringIndex returns [start, end]; keep access centralized
	// so scanner rules see the defensive checks before any position is used.
	if len(match) < 2 {
		return 0, false
	}
	for i, value := range match {
		if i == 0 {
			return value, true
		}
	}
	return 0, false
}
