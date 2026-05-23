package rules

import "testing"

func TestSLP126_FiresOnReferenceWithoutIndex(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/migrations/058_vps_connection_shares/migration.sql b/api/migrations/058_vps_connection_shares/migration.sql
--- a/api/migrations/058_vps_connection_shares/migration.sql
+++ b/api/migrations/058_vps_connection_shares/migration.sql
@@ -0,0 +1,5 @@
+CREATE TABLE vps_connection_shares (
+  id uuid primary key,
+  vps_connection_id uuid not null references vps_connections(id)
+);
`)
	got := SLP126{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected finding when reference column has no matching index")
	}
}

func TestSLP126_NoFireWhenIndexAdded(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/migrations/058_vps_connection_shares/migration.sql b/api/migrations/058_vps_connection_shares/migration.sql
--- a/api/migrations/058_vps_connection_shares/migration.sql
+++ b/api/migrations/058_vps_connection_shares/migration.sql
@@ -0,0 +1,7 @@
+CREATE TABLE vps_connection_shares (
+  id uuid primary key,
+  vps_connection_id uuid not null references vps_connections(id)
+);
+CREATE INDEX idx_vps_connection_shares_vps_connection_id ON vps_connection_shares (vps_connection_id);
`)
	got := SLP126{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when index is added, got %d", len(got))
	}
}

// Regression: index on tenant_id for table A must NOT suppress the finding for
// user_id on table B (the original cross-table false-negative).
func TestSLP126_TableScopedIndex_NoFalseNegative(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/migrations/121_dedup_groups/migration.sql b/api/migrations/121_dedup_groups/migration.sql
--- a/api/migrations/121_dedup_groups/migration.sql
+++ b/api/migrations/121_dedup_groups/migration.sql
@@ -0,0 +1,14 @@
+CREATE TABLE media_dedup_runs (
+  id              TEXT PRIMARY KEY,
+  tenant_id       TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
+  user_id         TEXT REFERENCES users(id) ON DELETE SET NULL
+);
+CREATE TABLE media_dedup_groups (
+  id          TEXT PRIMARY KEY,
+  run_id      TEXT NOT NULL REFERENCES media_dedup_runs(id) ON DELETE CASCADE,
+  tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE
+);
+CREATE INDEX idx_media_dedup_groups_tenant ON media_dedup_groups(tenant_id);
+CREATE INDEX idx_media_dedup_groups_run    ON media_dedup_groups(run_id);
`)
	// tenant_id and run_id on media_dedup_groups are indexed.
	// tenant_id and user_id on media_dedup_runs are NOT indexed — should fire.
	got := SLP126{}.Check(d)
	cols := map[string]bool{}
	for _, g := range got {
		cols[g.Message] = true
	}
	// Should fire for media_dedup_runs.tenant_id and media_dedup_runs.user_id
	if len(got) < 2 {
		t.Errorf("expected ≥2 findings (unindexed FK cols on media_dedup_runs), got %d: %v", len(got), got)
	}
}

// Index on the same column name in one table must NOT suppress a finding for
// that column on a different table with no index.
func TestSLP126_SameColumnDifferentTables(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/migrations/099_thing/migration.sql b/api/migrations/099_thing/migration.sql
--- a/api/migrations/099_thing/migration.sql
+++ b/api/migrations/099_thing/migration.sql
@@ -0,0 +1,12 @@
+CREATE TABLE parent_items (
+  id        TEXT PRIMARY KEY,
+  org_id    TEXT NOT NULL REFERENCES orgs(id)
+);
+CREATE INDEX idx_parent_items_org ON parent_items(org_id);
+CREATE TABLE child_items (
+  id        TEXT PRIMARY KEY,
+  org_id    TEXT NOT NULL REFERENCES orgs(id)
+);
`)
	// parent_items.org_id is indexed; child_items.org_id is NOT — should fire.
	got := SLP126{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected finding for child_items.org_id which has no index, even though parent_items.org_id is indexed")
	}
}

func TestSLP126_Description(t *testing.T) {
	r := SLP126{}
	if r.ID() != "SLP126" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityWarn {
		t.Errorf("default severity should be warn")
	}
}
