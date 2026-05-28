package rules

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP058 flags SQL strings built with string concatenation or interpolation.
type SLP058 struct{}

// ID returns the rule identifier: "SLP058".
func (SLP058) ID() string { return "SLP058" }

// DefaultSeverity returns this rule's default severity.
func (SLP058) DefaultSeverity() Severity { return SeverityBlock }

// Description returns a short description of the SLP058 rule.
func (SLP058) Description() string {
	return "SQL built with string concatenation"
}

// sqlConcatPatternStrict matches a concat-built SQL query where the keyword is
// in UPPERCASE — the conventional shape of real SQL queries
// (`"SELECT * FROM users WHERE id = " + userID`). Production SQL across the
// whimsy/codero corpora uses UPPERCASE keywords; lowercase occurrences are
// almost always English prose ("Backups run from a connector", "where users
// click") and are the primary source of SLP058 false positives in JSX/.tsx
// attribute strings.
var sqlConcatPatternStrict = regexp.MustCompile(`(?s)\b(SELECT|INSERT|UPDATE|DELETE|WHERE|FROM|JOIN)\b.*(\+|\$\{)`)

// sqlConcatPatternLoose matches the lowercase variant. We still flag it, but
// only when the line (or a nearby line in the same hunk) shows a recognizable
// SQL execution context — see sqlCallContextRe.
var sqlConcatPatternLoose = regexp.MustCompile(`(?is)\b(select|insert|update|delete|where|from|join)\b.*(\+|\$\{)`)

// sqlSprintfPattern keeps the original case-insensitive Sprintf match shape:
// `fmt.Sprintf("... %s ... FROM ... WHERE ...", ...)` style injection. Real
// fmt.Sprintf SQL injection is always paired with both a SQL keyword and a
// format verb in the same call, which is enough signal on its own.
var sqlSprintfPattern = regexp.MustCompile(`(?is)fmt\.Sprintf\s*\([^)]*(?:\b(select|insert|update|delete|where|from|join)\b[^)]*%[vTtbcdoqxXfFeEgGsp]|%[vTtbcdoqxXfFeEgGsp][^)]*\b(select|insert|update|delete|where|from|join)\b)[^)]*\)`)

// sqlCallContextRe matches the most common SQL execution contexts. Used to
// promote a lowercase-keyword line to a finding when a real query is clearly
// being assembled (e.g. `pool.query("select * from x where id = " + id)`).
var sqlCallContextRe = regexp.MustCompile(`\b(query|execute|prepare|exec|run|raw)\s*\(|\b(pool|db|client|cursor|knex|sqlx|conn|connection)\b\s*\.`)

func sqlConcatGoSafe(line string, loc []int) bool {
	// Defensive length guard so SLP118 (numeric-index access) reads this as
	// safe even though all callers already pass a non-nil regex match.
	if len(loc) == 0 {
		return true
	}
	prefix := line[:loc[0]]
	if strings.Count(prefix, "`")%2 == 1 {
		return false
	}
	if isInsideRegexpCall(prefix, loc[0]) {
		return false
	}
	return true
}

// Check implements the diff-aware SLP058 rule for SQL string concatenation.
func (r SLP058) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Path))
		if ext != ".go" && ext != ".js" && ext != ".jsx" && ext != ".ts" && ext != ".tsx" && !strings.HasSuffix(f.Path, ".py") {
			continue
		}
		isGo := ext == ".go"
		for _, h := range f.Hunks {
			// A SQL execution call ANYWHERE in the hunk (context or added)
			// is enough to admit lowercase-keyword lines — the query is being
			// assembled here regardless of casing.
			hunkHasSQLCtx := false
			for _, ln := range h.Lines {
				if sqlCallContextRe.MatchString(ln.Content) {
					hunkHasSQLCtx = true
					break
				}
			}
			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					continue
				}
				fire := false
				var loc []int
				switch {
				case sqlSprintfPattern.MatchString(ln.Content):
					fire = true
					loc = sqlSprintfPattern.FindStringIndex(ln.Content)
				case sqlConcatPatternStrict.MatchString(ln.Content):
					fire = true
					loc = sqlConcatPatternStrict.FindStringIndex(ln.Content)
				case sqlConcatPatternLoose.MatchString(ln.Content) &&
					(hunkHasSQLCtx || sqlCallContextRe.MatchString(ln.Content)):
					fire = true
					loc = sqlConcatPatternLoose.FindStringIndex(ln.Content)
				}
				if !fire || loc == nil {
					continue
				}
				if isGo && !sqlConcatGoSafe(ln.Content, loc) {
					continue
				}
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     ln.NewLineNo,
					Message:  "SQL built with string concatenation — use parameterized queries",
					Snippet:  strings.TrimSpace(ln.Content),
				})
			}
		}
	}
	return out
}

// isInsideRegexpCall checks whether the match at position matchPos in the
// line is actually inside a regexp.MustCompile() or regexp.Compile() call.
// Uses parentheses balancing so that a SQL match on the same line *after*
// the closing ) of the regexp call is not incorrectly suppressed.
func isInsideRegexpCall(prefix string, matchPos int) bool {
	for _, pat := range []string{"regexp.MustCompile(", "regexp.Compile("} {
		pos := strings.LastIndex(prefix, pat)
		if pos < 0 {
			continue
		}
		// Content between the opening paren of the regexp call and the match.
		between := prefix[pos+len(pat):]
		opens := strings.Count(between, "(")
		closes := strings.Count(between, ")")
		// If there are no unmatched closing parens, the match is inside the
		// regexp call (or nested inside a deeper call within it).
		if opens >= closes {
			return true
		}
	}
	return false
}
