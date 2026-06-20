package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP226 flags SQL resource and transaction imbalance in added code.
//
// 1. `sql.Rows` or `sql.Stmt` variables that are used in the hunk but not
//    paired with a `defer .Close()` in the same hunk.
//    Example flagged:
//
//	  rows, err := db.Query(...)
//	  if err != nil { return err }
//	  for rows.Next() { ... }   // no defer rows.Close() in hunk
//
// 2. `BEGIN` (or `db.BeginTx`) introduced in the hunk without a matching
//    `COMMIT` or `ROLLBACK` in the same hunk.
//
// The rule is diff-based and conservative. It only inspects added lines.
// It will not flag a variable that is returned to the caller or passed to
// another function.
//
// Severity: warn (heuristic).
type SLP226 struct{}

func (SLP226) ID() string                { return "SLP226" }
func (SLP226) DefaultSeverity() Severity { return SeverityWarn }
func (SLP226) Description() string {
	return "SQL resource or transaction not balanced in hunk"
}

// sqlVarRe matches `rows, err := db.Query(...)`, `stmt, err := db.Prepare(...)`,
// and single-return variants. Group 1 and 3 are the variable names.
var sqlVarRe = regexp.MustCompile(`(?m)(\w+)\s*,\s*\w*\s*:=\s*(?:[A-Za-z0-9_.]+\.)?(Query|QueryContext|Prepare|PrepareContext|Begin|BeginTx)\(|(\w+)\s*:=\s*(?:[A-Za-z0-9_.]+\.)?(Query|QueryContext|Prepare|PrepareContext|Begin|BeginTx)\(`)

// closeCallRe matches defer rows.Close() / defer stmt.Close() for a given var.
var closeCallRe = regexp.MustCompile(`defer\s+%s\.Close\(\s*\)`)

// txnCloseRe matches commit/rollback on a transaction variable.
var txnCloseRe = regexp.MustCompile(`%s\.(Commit|Rollback)\(\s*\)`)

func (r SLP226) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Path), "_test.go") {
			continue
		}

		for _, h := range f.Hunks {
			hunkText := ""
			addedText := ""
			var added []diff.Line
			for _, ln := range h.Lines {
				if ln.Kind == diff.LineAdd || ln.Kind == diff.LineContext {
					hunkText += ln.Content + "\n"
				}
				if ln.Kind == diff.LineAdd {
					addedText += ln.Content + "\n"
					added = append(added, ln)
				}
			}
			if len(added) == 0 {
				continue
			}

			// Only scan added lines for SQL variable assignments to avoid
			// flagging pre-existing resources that were not introduced by
			// this diff.
			vars := collectSQLVars(sqlVarRe, addedText)
			for _, v := range vars {
				// Determine whether this is a rows/stmt or a transaction.
				isTx := false
				beginRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(v) + `\s*,\s*\w*\s*:=\s*(?:[A-Za-z0-9_.]+\.)?Begin(?:Tx)?\(|\b` + regexp.QuoteMeta(v) + `\s*:=\s*(?:[A-Za-z0-9_.]+\.)?Begin(?:Tx)?\(`)
				if beginRe.MatchString(hunkText) {
					isTx = true
				}

				if isTx {
					pattern := fmtTxnClosePattern(v)
					re := regexp.MustCompile(pattern)
					if re.MatchString(hunkText) {
						continue
					}
					out = append(out, Finding{
						RuleID:   r.ID(),
						Severity: r.DefaultSeverity(),
						File:     f.Path,
						Line:     added[0].NewLineNo,
						Message:  "transaction " + v + " started without matching Commit/Rollback in this hunk",
						Snippet:  strings.TrimSpace(added[0].Content),
					})
					continue
				}

				pattern := fmtClosePattern(v)
				re := regexp.MustCompile(pattern)
				if re.MatchString(hunkText) {
					continue
				}
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     added[0].NewLineNo,
					Message:  "SQL " + v + " is used without defer " + v + ".Close() in this hunk",
					Snippet:  strings.TrimSpace(added[0].Content),
				})
			}
		}
	}
	return out
}

// collectSQLVars extracts variable names created by the given assignment
// pattern that lead to a SQLRows/Stmt/Tx-like call.
func collectSQLVars(re *regexp.Regexp, text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		// The regex has two alternatives. m[1] is the name from the two-value
		// form, m[3] is the name from the single-value form.
		for _, idx := range []int{1, 3} {
			if idx >= len(m) {
				continue
			}
			g := strings.TrimSpace(m[idx])
			if g == "" || g == "err" || seen[g] {
				continue
			}
			seen[g] = true
			out = append(out, g)
		}
	}
	return out
}

func fmtClosePattern(varName string) string {
	return strings.ReplaceAll(closeCallRe.String(), `%s`, regexp.QuoteMeta(varName))
}

func fmtTxnClosePattern(varName string) string {
	return strings.ReplaceAll(txnCloseRe.String(), `%s`, regexp.QuoteMeta(varName))
}
