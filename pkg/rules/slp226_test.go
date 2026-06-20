package rules

import (
	"testing"
)

func TestSLP226RowsWithoutClose(t *testing.T) {
	d := parseDiff(t, `diff --git a/store.go b/store.go
--- a/store.go
+++ b/store.go
@@ -10,5 +10,12 @@
 func List() ([]Item, error) {
+    rows, err := db.Query("SELECT id FROM items")
+    if err != nil { return nil, err }
+    var out []Item
+    for rows.Next() {
+        var it Item
+        _ = rows.Scan(&it.ID)
+        out = append(out, it)
+    }
+    return out, nil
 }
`)
	got := len(SLP226{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}
}

func TestSLP226RowsWithDeferCloseOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/store.go b/store.go
--- a/store.go
+++ b/store.go
@@ -10,5 +10,13 @@
 func List() ([]Item, error) {
     rows, err := db.Query("SELECT id FROM items")
     if err != nil { return nil, err }
+    defer rows.Close()
+    var out []Item
+    for rows.Next() {
+        var it Item
+        _ = rows.Scan(&it.ID)
+        out = append(out, it)
+    }
+    return out, nil
 }
`)
	got := len(SLP226{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings with defer close, got %d", got)
	}
}

func TestSLP226StmtWithoutClose(t *testing.T) {
	d := parseDiff(t, `diff --git a/store.go b/store.go
--- a/store.go
+++ b/store.go
@@ -10,5 +10,10 @@
 func upsert(db *sql.DB) error {
+    stmt, err := db.Prepare("INSERT ...")
+    if err != nil { return err }
+    _, err = stmt.Exec("x")
+    return err
 }
`)
	got := len(SLP226{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}
}

func TestSLP226TransactionImbalance(t *testing.T) {
	d := parseDiff(t, `diff --git a/store.go b/store.go
--- a/store.go
+++ b/store.go
@@ -10,5 +10,10 @@
 func withTx(db *sql.DB) error {
+    tx, err := db.Begin()
+    if err != nil { return err }
+    _, err = tx.Exec("INSERT ...")
+    return err
 }
`)
	got := len(SLP226{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}
}

func TestSLP226TransactionBalancedOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/store.go b/store.go
--- a/store.go
+++ b/store.go
@@ -10,5 +10,16 @@
 func withTx(db *sql.DB) error {
     tx, err := db.Begin()
     if err != nil { return err }
+    defer func() {
+        if err != nil {
+            _ = tx.Rollback()
+            return
+        }
+        _ = tx.Commit()
+    }()
+    _, err = tx.Exec("INSERT ...")
+    return err
 }
`)
	got := len(SLP226{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings with rollback/commit, got %d", got)
	}
}
