package rules

import (
	"testing"
)

func TestSLP224BodyReadMissingValidation(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -10,5 +10,8 @@
 func Handler(w http.ResponseWriter, r *http.Request) {
     var body Req
+    json.NewDecoder(r.Body).Decode(&body)
 }
`)
	got := len(SLP224{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}
}

func TestSLP224ContentLengthGuardOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -10,5 +10,12 @@
 func Handler(w http.ResponseWriter, r *http.Request) {
     var body Req
+    if r.ContentLength > 0 {
+        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
+            http.Error(w, err.Error(), http.StatusBadRequest)
+            return
+        }
+    }
 }
`)
	got := len(SLP224{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings with guards, got %d", got)
	}
}

func TestSLP224DecodeErrorGuardOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -10,5 +10,11 @@
 func Handler(w http.ResponseWriter, r *http.Request) {
     var body Req
+    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
+        http.Error(w, err.Error(), http.StatusBadRequest)
+        return
+    }
 }
`)
	got := len(SLP224{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings with decode-error handling, got %d", got)
	}
}

func TestSLP224NonHandlerNotFlagged(t *testing.T) {
	d := parseDiff(t, `diff --git a/util.go b/util.go
--- a/util.go
+++ b/util.go
@@ -10,5 +10,8 @@
 func parseBody(r io.Reader) (*Req, error) {
     var body Req
+    if err := json.NewDecoder(r).Decode(&body); err != nil {
+        return nil, err
+    }
 }
`)
	got := len(SLP224{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for non-http.Request helper, got %d", got)
	}
}
