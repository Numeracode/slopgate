package rules

import (
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP126 flags migration SQL that introduces FK/reference columns without a
// matching CREATE INDEX in the same diff. Checks are table-scoped so that an
// index on column X in table A does not suppress a warning about column X in
// table B.
type SLP126 struct{}

func (SLP126) ID() string                { return "SLP126" }
func (SLP126) DefaultSeverity() Severity { return SeverityWarn }
func (SLP126) Description() string {
	return "migration adds FK/reference column without index — add CREATE INDEX for join/cascade performance"
}

// matches a line that defines a FK/reference: inline REFERENCES, FOREIGN KEY
// clause, or ADD COLUMN *_id.
var slp126RefLineRe = regexp.MustCompile(
	`(?i)(foreign\s+key|references\s+[a-z0-9_"]+|\badd\s+column(?:\s+if\s+not\s+exists)?\s+[a-z0-9_"]+_id\b)`)

// extracts *_id column tokens from a line.
var slp126IDTokenRe = regexp.MustCompile(`(?i)\b([a-z0-9_]+_id)\b`)

// matches a CREATE INDEX / ADD INDEX line.
var slp126IndexLineRe = regexp.MustCompile(`(?i)\b(create\s+(unique\s+)?index|add\s+index)`)

// extracts the table name from a CREATE INDEX … ON <table>(…) line.
var slp126IndexOnTableRe = regexp.MustCompile(`(?i)\bon\s+([a-z0-9_"]+)\s*\(`)

// matches a CREATE TABLE / ALTER TABLE line to track the current table context.
var slp126TableRe = regexp.MustCompile(`(?i)\b(create\s+table(?:\s+if\s+not\s+exists)?|alter\s+table)\s+([a-z0-9_"]+)`)

func slp126IsMigrationSQL(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".sql") &&
		(strings.Contains(lower, "migration") || strings.Contains(lower, "migrations"))
}

type slp126Hit struct {
	table   string
	column  string
	line    diff.Line
	snippet string
}

func (r SLP126) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || !slp126IsMigrationSQL(f.Path) {
			continue
		}

		// table → set of indexed columns
		indexedByTable := map[string]map[string]bool{}
		var candidates []slp126Hit
		currentTable := ""

		for _, ln := range f.AddedLines() {
			content := strings.TrimSpace(stripCommentAndStrings(ln.Content))
			if content == "" {
				continue
			}
			lower := strings.ToLower(content)

			// Track current CREATE TABLE / ALTER TABLE context.
			if m := slp126TableRe.FindStringSubmatch(lower); m != nil {
				currentTable = strings.Trim(m[2], `"`)
			}

			// Track CREATE INDEX … ON table(col, …).
			if slp126IndexLineRe.MatchString(lower) {
				tableName := currentTable
				if tm := slp126IndexOnTableRe.FindStringSubmatch(lower); tm != nil {
					tableName = strings.Trim(tm[1], `"`)
				}
				if tableName != "" {
					if indexedByTable[tableName] == nil {
						indexedByTable[tableName] = map[string]bool{}
					}
					for _, m := range slp126IDTokenRe.FindAllStringSubmatch(lower, -1) {
						if len(m) == 2 {
							indexedByTable[tableName][m[1]] = true
						}
					}
				}
				continue
			}

			// Detect FK/reference lines.
			if !slp126RefLineRe.MatchString(lower) {
				continue
			}
			for _, m := range slp126IDTokenRe.FindAllStringSubmatch(lower, -1) {
				if len(m) != 2 {
					continue
				}
				candidates = append(candidates, slp126Hit{
					table:   currentTable,
					column:  m[1],
					line:    ln,
					snippet: strings.TrimSpace(ln.Content),
				})
			}
		}

		if len(candidates) == 0 {
			continue
		}

		seen := map[string]bool{}
		for _, c := range candidates {
			key := c.table + "." + c.column
			if seen[key] {
				continue
			}
			seen[key] = true
			// Check table-scoped index coverage; also accept a global (empty
			// table key) index as a fallback for ALTER TABLE contexts.
			tableIdxs := indexedByTable[c.table]
			globalIdxs := indexedByTable[""]
			if tableIdxs[c.column] || globalIdxs[c.column] {
				continue
			}
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.DefaultSeverity(),
				File:     f.Path,
				Line:     c.line.NewLineNo,
				Message: "migration adds FK column '" + c.column + "' on table '" + c.table +
					"' without a matching index — add CREATE INDEX for join/cascade performance",
				Snippet: c.snippet,
			})
		}
	}
	return out
}
