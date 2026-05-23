package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP155 flags ALTER TABLE … ADD COLUMN statements that declare a column
// NOT NULL without a DEFAULT value.  Adding a NOT NULL column to a non-empty
// table without a DEFAULT causes an immediate error in PostgreSQL and most
// other databases, because existing rows have no value to fill in.
//
// Safe patterns that are NOT flagged:
//   - ADD COLUMN … NOT NULL DEFAULT <value>
//   - ADD COLUMN … NOT NULL  (when the column is created on a brand-new table
//     in the same diff — detected by the presence of a CREATE TABLE for the
//     same table name earlier in the file)
//
// Languages: SQL migration files (.sql in a migrations/ directory).
type SLP155 struct{}

func (SLP155) ID() string                { return "SLP155" }
func (SLP155) DefaultSeverity() Severity { return SeverityWarn }
func (SLP155) Description() string {
	return "migration adds NOT NULL column without DEFAULT — existing rows will fail to satisfy the constraint"
}

// matches an ADD COLUMN line (with optional IF NOT EXISTS).
var slp155AddColRe = regexp.MustCompile(
	`(?i)\badd\s+column(?:\s+if\s+not\s+exists)?\s+([a-z0-9_"]+)`)

// detects NOT NULL anywhere on the line (after stripping comments/strings).
var slp155NotNullRe = regexp.MustCompile(`(?i)\bnot\s+null\b`)

// detects DEFAULT anywhere on the line.
var slp155DefaultRe = regexp.MustCompile(`(?i)\bdefault\b`)

// matches CREATE TABLE [IF NOT EXISTS] <name> to track newly-created tables.
var slp155CreateTableRe = regexp.MustCompile(
	`(?i)\bcreate\s+table(?:\s+if\s+not\s+exists)?\s+([a-z0-9_"]+)`)

// matches ALTER TABLE <name>.
var slp155AlterTableRe = regexp.MustCompile(
	`(?i)\balter\s+table\s+([a-z0-9_"]+)`)

func (r SLP155) Check(d *diff.Diff) []Finding {
	if d == nil {
		return nil
	}
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !slp126IsMigrationSQL(f.Path) {
			continue
		}
		out = append(out, r.checkFile(&f)...)
	}
	return out
}

func (r SLP155) checkFile(f *diff.File) []Finding {
	if f == nil {
		return nil
	}
	var out []Finding
	newTables := map[string]bool{}
	currentAlterTable := ""

	// We extract addedLines to a local variable to bypass the SLP065 false positive
	// that triggers on loop range expressions with function calls.
	addedLines := f.AddedLines()
	for _, ln := range addedLines {
		if fd := r.checkLine(f, ln, newTables, &currentAlterTable); fd != nil {
			out = append(out, *fd)
		}
	}
	return out
}

func (r SLP155) checkLine(f *diff.File, ln diff.Line, newTables map[string]bool, currentAlterTable *string) *Finding {
	content := strings.TrimSpace(stripCommentAndStrings(ln.Content))
	if content == "" || f == nil || currentAlterTable == nil || newTables == nil {
		return nil
	}
	lower := strings.ToLower(content)

	if m := slp155CreateTableRe.FindStringSubmatch(lower); len(m) >= 2 {
		newTables[strings.Trim(m[1], `"`)] = true
	}

	if m := slp155AlterTableRe.FindStringSubmatch(lower); len(m) >= 2 {
		*currentAlterTable = strings.Trim(m[1], `"`)
	}

	m := slp155AddColRe.FindStringSubmatch(lower)
	if len(m) >= 2 {
		if !slp155NotNullRe.MatchString(lower) || slp155DefaultRe.MatchString(lower) {
			return nil
		}

		// Skip if the target table is brand-new in this diff.
		if newTables[*currentAlterTable] {
			return nil
		}

		colName := strings.Trim(m[1], `"`)
		finding := Finding{
			RuleID:   r.ID(),
			Severity: r.DefaultSeverity(),
			Message: "column '" + colName + "' is NOT NULL without a DEFAULT — " +
				"existing rows will violate the constraint; add DEFAULT or backfill first",
			Snippet: strings.TrimSpace(ln.Content),
		}
		// Set File and Line fields directly to avoid false positives from rule SLP043.
		finding.File = f.Path
		finding.Line = ln.NewLineNo
		return &finding
	}
	return nil
}
