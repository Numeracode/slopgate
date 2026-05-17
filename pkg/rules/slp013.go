package rules

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP013 flags runs of three or more consecutive added lines that look
// like commented-out code (as opposed to ordinary prose comments).
//
// Rationale: AI agents often leave their previous attempt commented
// out "just in case" when they rewrite a block. Committed dead code
// is slop that rots the file and confuses the next reader. Ordinary
// multi-line prose comments are exempt.
type SLP013 struct{}

func (SLP013) ID() string                { return "SLP013" }
func (SLP013) DefaultSeverity() Severity { return SeverityBlock }
func (SLP013) Description() string {
	return "block of commented-out code added in new diff"
}

// minBlockSize is the smallest run length that is considered a "block".
// Two lines is often a legitimate diff review artifact; three is where
// intent becomes suspicious.
const slp013MinBlockSize = 3

// slp013CommentPrefix matches the leading comment marker on a full-line
// comment and captures the body (content after the marker) for
// code-shape testing.
//
// Note: SQL/Lua `--` and Lisp `;` are deliberately omitted. `--` at
// line start collides with CSS custom properties (e.g. `--bg-base: …`)
// and `;` collides with statement terminators on wrapped diff lines.
// Support for those languages is a v0.0.2 concern.
var slp013CommentPrefix = regexp.MustCompile(`^\s*(?://\s?|#\s?|\*\s+)(.*)$`)

// strongAnywhereTokens are substrings that strongly suggest code even
// when they appear mid-line. We keep this list short and discriminating;
// anything that can plausibly appear in English prose was removed.
var strongAnywhereTokens = []string{
	":=", "->", "=>", "::",
}

// strongPrefixKeywords are statement-starting keywords in common
// languages. If the first word of a comment body is one of these, it
// is probably code. Using first-word matching instead of substring
// matching keeps prose like "for the underlying instance" from looking
// like Go `for`.
var strongPrefixKeywords = map[string]bool{
	"return": true, "if": true, "else": true, "for": true, "while": true,
	"switch": true, "case": true, "break": true, "continue": true,
	"func": true, "def": true, "class": true, "struct": true, "interface": true,
	"var": true, "const": true, "let": true, "import": true, "package": true,
	"try": true, "catch": true, "throw": true, "raise": true, "with": true,
}

// slp013InlineCode matches a backtick-delimited span. Inline code in a
// prose comment is conventionally backtick-quoted, so these spans are
// stripped before code-shape testing — quoting `=>` or `foo()` in a
// sentence must not make the sentence look like commented-out code.
var slp013InlineCode = regexp.MustCompile("`[^`]*`")

// functionCallPattern matches an identifier immediately followed by an
// opening paren — the canonical shape of a function call. Prose with
// parentheticals ("(e.g. foo)") has no identifier glued to the `(`,
// so it does not match.
var functionCallPattern = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*\(([^)]*)\)`)

// maxCodeShapeWords is the word-count cutoff above which a comment line
// is considered prose regardless of any code-ish tokens it contains.
// Real commented-out code is terse; English explanation is not.
const maxCodeShapeWords = 10

// isCodeShapedCommentBody reports whether the given body looks like
// a line of code rather than prose.
func isCodeShapedCommentBody(body string) bool {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return false
	}
	// Drop backtick-quoted inline code so a prose comment that quotes a
	// token (e.g. "the `=>` form") is not mistaken for real code. A Go
	// raw-string literal that is genuinely commented out still keeps its
	// surrounding statement shape (`q := ` ...) outside the backticks.
	if strings.Contains(trimmed, "`") {
		trimmed = strings.TrimSpace(slp013InlineCode.ReplaceAllString(trimmed, " "))
		if trimmed == "" {
			return false
		}
	}
	// Bullet-list prefixes indicate a documentation example, not a
	// commented-out statement, even if the bullet content contains
	// function-call syntax.
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "1. ") {
		return false
	}
	words := strings.Fields(trimmed)
	if len(words) > maxCodeShapeWords {
		return false
	}

	// Ends-with structural tokens.
	if strings.HasSuffix(trimmed, "{") || strings.HasSuffix(trimmed, "}") || strings.HasSuffix(trimmed, ";") {
		return true
	}

	// Strong anywhere tokens.
	for _, tok := range strongAnywhereTokens {
		if strings.Contains(trimmed, tok) {
			return true
		}
	}

	// First word is a statement-starting keyword (with trailing
	// punctuation stripped).
	first := strings.TrimFunc(words[0], func(r rune) bool {
		return r == ':' || r == ','
	})
	if strongPrefixKeywords[first] {
		return true
	}

	// Function-call shape with a compact argument list. We match an
	// identifier glued to `(` and require the captured inner text to
	// be short and free of obvious prose markers.
	if m := functionCallPattern.FindStringSubmatch(trimmed); m != nil {
		inner := m[1]
		if len(inner) <= 40 && !strings.Contains(inner, "e.g.") && !strings.Contains(inner, "i.e.") {
			return true
		}
	}
	return false
}

// slp013SkipExtensions lists file types where SLP013 should not run.
// Config and data formats have their own comment semantics and are
// out of scope for "commented-out code" detection.
var slp013SkipExtensions = map[string]bool{
	".yml":  true,
	".yaml": true,
	".toml": true,
	".json": true,
	".ini":  true,
	".conf": true,
	".cfg":  true,
	".env":  true,
}

func (r SLP013) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) {
			continue
		}
		if slp013SkipExtensions[strings.ToLower(filepath.Ext(f.Path))] {
			continue
		}
		for _, h := range f.Hunks {
			// Walk consecutive added lines. A run is broken by any
			// non-Add line or any added line that isn't a full-line
			// comment.
			type commentLine struct {
				body string
				no   int
			}
			var run []commentLine
			flush := func() {
				defer func() { run = nil }()
				if len(run) < slp013MinBlockSize {
					return
				}
				codey := 0
				for _, c := range run {
					if isCodeShapedCommentBody(c.body) {
						codey++
					}
				}
				// Require a majority of the block to look code-shaped
				// so prose blocks of the same length don't fire.
				if codey*2 <= len(run) {
					return
				}
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     run[0].no,
					Message:  fmt.Sprintf("block of %d commented-out code lines added — delete it or restore it", len(run)),
					Snippet:  strings.TrimSpace(run[0].body),
				})
			}

			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					flush()
					continue
				}
				m := slp013CommentPrefix.FindStringSubmatch(ln.Content)
				if m == nil {
					flush()
					continue
				}
				run = append(run, commentLine{body: m[1], no: ln.NewLineNo})
			}
			flush()
		}
	}
	return out
}
