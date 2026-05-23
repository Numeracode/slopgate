package rules

import "testing"

func TestSLP155_FiresOnNotNullWithoutDefault(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/migrations/122_add_status/migration.sql b/api/migrations/122_add_status/migration.sql
--- a/api/migrations/122_add_status/migration.sql
+++ b/api/migrations/122_add_status/migration.sql
@@ -0,0 +1,3 @@
+ALTER TABLE jobs
+  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL;
`)
	got := SLP155{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected finding: NOT NULL column without DEFAULT on existing table")
	}
	if got[0].RuleID != "SLP155" {
		t.Errorf("unexpected rule ID %q", got[0].RuleID)
	}
}

func TestSLP155_NoFireWhenDefaultPresent(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/migrations/122_add_status/migration.sql b/api/migrations/122_add_status/migration.sql
--- a/api/migrations/122_add_status/migration.sql
+++ b/api/migrations/122_add_status/migration.sql
@@ -0,0 +1,3 @@
+ALTER TABLE jobs
+  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'pending';
`)
	got := SLP155{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when DEFAULT is present, got %d", len(got))
	}
}

func TestSLP155_NoFireWhenNullable(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/migrations/122_add_notes/migration.sql b/api/migrations/122_add_notes/migration.sql
--- a/api/migrations/122_add_notes/migration.sql
+++ b/api/migrations/122_add_notes/migration.sql
@@ -0,0 +1,2 @@
+ALTER TABLE jobs ADD COLUMN notes TEXT;
`)
	got := SLP155{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for nullable column, got %d", len(got))
	}
}

// Brand-new table: ADD COLUMN NOT NULL on a table created in the same diff is safe.
func TestSLP155_NoFireOnNewTable(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/migrations/123_new_table/migration.sql b/api/migrations/123_new_table/migration.sql
--- a/api/migrations/123_new_table/migration.sql
+++ b/api/migrations/123_new_table/migration.sql
@@ -0,0 +1,7 @@
+CREATE TABLE new_things (
+  id TEXT PRIMARY KEY
+);
+ALTER TABLE new_things
+  ADD COLUMN name TEXT NOT NULL;
`)
	got := SLP155{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings on newly-created table, got %d: %v", len(got), got)
	}
}

// Non-migration SQL files should be ignored.
func TestSLP155_IgnoresNonMigrationFile(t *testing.T) {
	d := parseDiff(t, `diff --git a/scripts/seed.sql b/scripts/seed.sql
--- a/scripts/seed.sql
+++ b/scripts/seed.sql
@@ -0,0 +1,2 @@
+ALTER TABLE jobs ADD COLUMN status TEXT NOT NULL;
`)
	got := SLP155{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for non-migration SQL, got %d", len(got))
	}
}

func TestSLP155_Description(t *testing.T) {
	r := SLP155{}
	if r.ID() != "SLP155" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityWarn {
		t.Errorf("default severity = %v, want warn", r.DefaultSeverity())
	}
}
