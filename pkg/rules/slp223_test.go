package rules

import (
	"testing"
)

func TestSLP223IgnoredClose(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,8 @@
 func run() error {
     f, err := os.Open("input")
     if err != nil { return err }
+    _ = f.Close()
 }
`)
	got := len(SLP223{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}
}

func TestSLP223IgnoredMkdir(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,8 @@
 func setup() error {
+    _ = os.Mkdir("cache", 0755)
 }
`)
	got := len(SLP223{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}
}

func TestSLP223IgnoredRowsClose(t *testing.T) {
	d := parseDiff(t, `diff --git a/store.go b/store.go
--- a/store.go
+++ b/store.go
@@ -10,5 +10,8 @@
 func list() error {
     rows, err := db.Query("SELECT id FROM items")
     if err != nil { return err }
+    _ = rows.Close()
 }
`)
	got := len(SLP223{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}
}

func TestSLP223DeferredEncoderOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.go b/handler.go
--- a/handler.go
+++ b/handler.go
@@ -10,5 +10,12 @@
 func Handler(w http.ResponseWriter, r *http.Request) {
     defer func() {
+        _ = json.NewEncoder(w).Encode(resp)
     }()
 }
`)
	got := len(SLP223{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for deferred encoder, got %d", got)
	}
}

func TestSLP223ReturnedErrorOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,8 @@
 func run() error {
-    return doThing()
+    err := doThing()
+    if err != nil { return err }
+    return nil
 }
`)
	got := len(SLP223{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for handled error, got %d", got)
	}
}
