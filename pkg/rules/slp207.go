package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
	"path/filepath"
)

// SLP207 flags code paths where a database transaction is started but
// no explicit rollback is present on error return paths.
//
// Primary pattern (high signal): a BEGIN / db.Begin() is added but the
// corresponding ROLLBACK / tx.Rollback() is missing, and an error-return
// or error-propagation path exists in the same hunk.
//
// This catches the common Sentry bug: a transaction is left open/abandoned
// when an error occurs before commit, causing connection leaks or
// inconsistent state.
//
// Languages: Go, Python, Java, JS/TS, SQL.
// Scope: diff only — scans added lines within each file hunk.
//
// JS/TS Drizzle transactions are callback-managed (Drizzle issues ROLLBACK
// itself when the callback rejects). A begin is exempted only when
// slp207_managed.go proves, against the scanned source snapshot, that this
// specific begin is such a managed call on a Drizzle receiver. Proven managed
// calls are excluded individually; their callback contents are never used as
// rollback/commit evidence for any other begin in the same hunk.
type SLP207 struct{}

// ID returns the rule identifier: "SLP207".
func (SLP207) ID() string { return "SLP207" }

// DefaultSeverity returns this rule's default severity.
func (SLP207) DefaultSeverity() Severity { return SeverityBlock }

// Description returns a short description of the SLP207 rule.
func (SLP207) Description() string {
	return "transaction started without explicit rollback on error path"
}

// ---------------------------------------------------------------------------
// Regex library
// ---------------------------------------------------------------------------

// slp207JSTransactionBegin is the JS/TS knex/sequelize/Objection/Drizzle
// transaction start. It is named so the managed-transaction recognizer can
// require that it is the only begin form on a line.
var slp207JSTransactionBegin = regexp.MustCompile(`(?i)(?:knex|db|sequelize)\s*\.\s*transaction\s*\(`)

// txBeginPatterns match the start of a database transaction.
var txBeginPatterns = []*regexp.Regexp{
	// Go: tx, err := db.Begin(ctx) / tx, err = db.Begin(ctx)
	regexp.MustCompile(`(?i)(?:var|:=|=)\s+[\w]+\s*,?\s*err\s*(?::=|=)\s*.*\.Begin\(`),
	// Go: db.Begin(ctx) without capturing tx (standalone call — rare but covers edge cases)
	regexp.MustCompile(`(?i)\.Begin\(\s*(?:context\.\w+|ctx|\w+Ctx)?\s*\)`),
	// Python: cursor.execute("BEGIN")
	regexp.MustCompile(`(?i)execute\s*\(\s*["']\s*BEGIN\s*["']`),
	// Python: connection.begin()
	regexp.MustCompile(`(?i)\.begin\s*\(`),
	// Java: connection.setAutoCommit(false) marks an explicit transaction boundary.
	regexp.MustCompile(`(?i)setAutoCommit\s*\(\s*false\s*\)`),
	// JS/TS: knex/sequelize/Objection transaction starts.
	slp207JSTransactionBegin,
	// Bare SQL BEGIN statement.
	regexp.MustCompile(`(?i)^\s*BEGIN\b`),
}

// txRollbackPatterns match an explicit transaction rollback.
var txRollbackPatterns = []*regexp.Regexp{
	// Go: tx.Rollback(ctx) / tx.Rollback()
	regexp.MustCompile(`(?i)\.Rollback\s*\(`),
	// Python: connection.rollback() / cursor.connection.rollback()
	regexp.MustCompile(`(?i)\.rollback\s*\(`),
	// Java: connection.rollback()
	regexp.MustCompile(`(?i)\.rollback\s*\(`),
	// JS/TS: trx.rollback() / knex.rollback()
	regexp.MustCompile(`(?i)\.rollback\s*\(`),
	// Bare SQL ROLLBACK.
	regexp.MustCompile(`(?i)^\s*ROLLBACK\b`),
}

// txCommitPatterns match an explicit transaction commit.
var txCommitPatterns = []*regexp.Regexp{
	// Go: tx.Commit() / tx.Commit(ctx)
	regexp.MustCompile(`(?i)\.Commit\s*\(`),
	// Python: connection.commit()
	regexp.MustCompile(`(?i)\.commit\s*\(`),
	// Java: connection.commit()
	regexp.MustCompile(`(?i)\.commit\s*\(`),
	// JS/TS: trx.commit() / knex.commit()
	regexp.MustCompile(`(?i)\.commit\s*\(`),
	// Bare SQL COMMIT.
	regexp.MustCompile(`(?i)^\s*COMMIT\b`),
}

// txOpPatterns match database operations that use the transaction.
// These distinguish "Begin failed, return err" (benign) from
// "Begin succeeded, ran queries, then returned error without rollback" (bug).
var txOpPatterns = []*regexp.Regexp{
	// Go: tx.Exec/Query/QueryRow/Call/Prepare (incl. *Context variants).
	// Pattern uses \w* suffix to match both Exec and ExecContext, etc.
	regexp.MustCompile(`(?i)\.(Exec|Query|QueryRow|Call|Prepare)\w*\s*\(`),
	// Python: cursor.execute(...)
	regexp.MustCompile(`(?i)\bexecute\s*\(`),
	// Java: ps.executeUpdate(), ps.executeQuery(), ps.execute()
	regexp.MustCompile(`(?i)\bexecute(Update|Query)?\s*\(`),
	// JS/TS: db.query(...), db.execute(...)
	regexp.MustCompile(`(?i)(?:db|conn|trx)\s*\.\s*(?:query|execute)\s*\(`),
}

// errorReturnPatterns match lines that return an error or propagate failure.
var errorReturnPatterns = []*regexp.Regexp{
	// Go: return err / return fmt.Errorf(...) / return errors.New(...)
	regexp.MustCompile(`(?i)\breturn\s+(err\w*|fmt\.Errorf|errors\.New)`),
	// Python: raise / return None (in transaction context)
	regexp.MustCompile(`(?i)\b(raise|return\s+None)\b`),
	// Java: throw new RuntimeException / throw e
	regexp.MustCompile(`(?i)\bthrow\s+(new\s+)?\w+`),
	// JS/TS: return res.status(500) / return { error: ... }
	regexp.MustCompile(`(?i)\breturn\s+(?:res\.status\(|res\.json\(|{[^}]*error)`),
}

// txSkipPatterns are lines we should never flag.
var txSkipPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\s*//`),  // Go/JS/TS/Java comment
	regexp.MustCompile(`^\s*#`),   // Python comment
	regexp.MustCompile(`^\s*/\*`), // block comment start
	regexp.MustCompile(`^\s*\*/`), // block comment end
	regexp.MustCompile(`^\s*\*`),  // doc comment line
	regexp.MustCompile(`^\s*$`),   // blank/whitespace-only line
}

// ---------------------------------------------------------------------------
// Check
// ---------------------------------------------------------------------------

// Check implements the diff-aware SLP207 rule for transaction rollback detection.
//
// Two conditions trigger a finding:
//
//  1. Error-path rollback gap: BEGIN + DB operations + error return, without ROLLBACK.
//     Catches the common Sentry bug where a transaction is started, queries run,
//     and an error is returned without rolling back.
//
//  2. Abandoned transaction: BEGIN without any COMMIT or ROLLBACK in the same hunk.
//     Catches forgotten transactions where no cleanup is attempted at all.
func (r SLP207) Check(d *diff.Diff) []Finding {
	var out []Finding
	managed := newSLP207Recognizer(d)

	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}

		ext := strings.ToLower(filepath.Ext(f.Path))
		if !slp207SupportedExt(ext) {
			continue
		}

		if isTestFile(f.Path) {
			continue
		}

		for _, h := range f.Hunks {
			addedLines := []addedLine{}
			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					continue
				}
				trimmed := strings.TrimSpace(ln.Content)
				if isTxSkippableLine(trimmed) {
					continue
				}
				addedLines = append(addedLines, addedLine{
					content:   trimmed,
					newLineNo: ln.NewLineNo,
				})
			}

			if len(addedLines) == 0 {
				continue
			}

			// Check: was a transaction started in this hunk?
			begins := findLineMatches(addedLines, txBeginPatterns)
			if len(begins) == 0 {
				continue
			}

			// Exclude begins proven to be callback-managed Drizzle
			// transactions, one begin at a time.
			var spans [][2]int
			var unmanaged []addedLine
			for _, bl := range begins {
				if start, end, ok := managed.managedSpan(f.Path, bl); ok {
					spans = append(spans, [2]int{start, end})
					continue
				}
				unmanaged = append(unmanaged, bl)
			}
			if len(unmanaged) == 0 {
				continue
			}
			begins = unmanaged

			// Rollback/commit evidence for the remaining begins must not come
			// from inside a managed callback: a `tx.rollback()` there belongs to
			// Drizzle's transaction, not to a sibling begin. With no managed
			// spans this is exactly the hunk's added lines, as before.
			evidence := slp207OutsideSpans(addedLines, spans)

			// Check if rollback or commit already exists in this hunk.
			hasRollback := hasAnyMatch(evidence, txRollbackPatterns)
			hasCommit := hasAnyMatch(evidence, txCommitPatterns)
			if hasRollback {
				continue
			}

			hasDBOps := hasAnyMatch(addedLines, txOpPatterns)
			hasErrorReturn := len(findLineMatches(addedLines, errorReturnPatterns)) > 0

			// Condition 1: error return after DB operations without rollback.
			// This is the main Sentry bug pattern.
			cond1 := hasDBOps && hasErrorReturn

			// Condition 2: abandoned transaction — BEGIN with no commit and no rollback.
			// The transaction was started but never cleaned up.
			cond2 := !hasCommit

			if !cond1 && !cond2 {
				continue
			}

			// Report one finding per begin line.
			reported := map[int]bool{}
			for _, bl := range begins {
				if reported[bl.newLineNo] {
					continue
				}
				reported[bl.newLineNo] = true
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     bl.newLineNo,
					Message:  "transaction started without explicit rollback on error path",
					Snippet:  bl.content,
				})
			}
		}
	}

	return out
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type addedLine struct {
	content   string
	newLineNo int
}

// slp207OutsideSpans returns the lines that fall outside every [start, end]
// source line span.
func slp207OutsideSpans(lines []addedLine, spans [][2]int) []addedLine {
	if len(spans) == 0 {
		return lines
	}
	var out []addedLine
	for _, ln := range lines {
		inside := false
		for _, sp := range spans {
			if ln.newLineNo >= sp[0] && ln.newLineNo <= sp[1] {
				inside = true
				break
			}
		}
		if !inside {
			out = append(out, ln)
		}
	}
	return out
}

// findLineMatches returns all added lines that match any of the given patterns.
func findLineMatches(lines []addedLine, patterns []*regexp.Regexp) []addedLine {
	var matches []addedLine
	for _, ln := range lines {
		for _, pat := range patterns {
			if pat.MatchString(ln.content) {
				matches = append(matches, ln)
				break
			}
		}
	}
	return matches
}

// hasAnyMatch returns true if any added line matches any of the given patterns.
func hasAnyMatch(lines []addedLine, patterns []*regexp.Regexp) bool {
	for _, ln := range lines {
		for _, pat := range patterns {
			if pat.MatchString(ln.content) {
				return true
			}
		}
	}
	return false
}

// isTxSkippableLine returns true for lines that should never be flagged by SLP07.
func isTxSkippableLine(content string) bool {
	trimmed := strings.TrimLeft(content, " \t")
	for _, pat := range txSkipPatterns {
		if pat.MatchString(trimmed) {
			return true
		}
	}
	return false
}

// slp207SupportedExt returns true for languages SLP207 covers.
func slp207SupportedExt(ext string) bool {
	switch ext {
	case ".go", ".js", ".ts", ".tsx", ".jsx", ".py", ".java", ".kt", ".rs", ".sql":
		return true
	}
	return false
}
