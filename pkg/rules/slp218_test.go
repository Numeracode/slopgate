package rules

import (
	"testing"
)

func TestSLP218ContentLengthGate(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -10,5 +10,8 @@
 func Handler(w http.ResponseWriter, r *http.Request) {
     var body Req
+    if r.ContentLength > 0 {
+        _ = json.NewDecoder(r.Body).Decode(&body)
+    }
 }
`)
	got := len(SLP218{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}
}

func TestSLP218TransferEncodingOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -10,5 +10,11 @@
 func Handler(w http.ResponseWriter, r *http.Request) {
     var body Req
+    if r.ContentLength > 0 || len(r.TransferEncoding) > 0 {
+        _ = json.NewDecoder(r.Body).Decode(&body)
+    }
 }
`)
	got := len(SLP218{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings when TransferEncoding referenced, got %d", got)
	}
}

func TestSLP218FileOpaquePath(t *testing.T) {
	d := parseDiff(t, `diff --git a/uri.go b/uri.go
--- a/uri.go
+++ b/uri.go
@@ -10,5 +10,8 @@
 func FileURL(path string) *url.URL {
-    return &url.URL{Scheme: "file", Path: path}
+    return &url.URL{Scheme: "file", Opaque: path}
 }
`)
	got := len(SLP218{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding for Opaque path, got %d", got)
	}
}

func TestSLP218FilePathOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/uri.go b/uri.go
--- a/uri.go
+++ b/uri.go
@@ -10,5 +10,8 @@
 func FileURL(path string) *url.URL {
-    return &url.URL{Scheme: "file", Opaque: path}
+    return &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
 }
`)
	got := len(SLP218{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for Path usage, got %d", got)
	}
}

func TestSLP218NonPathOpaqueOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/uri.go b/uri.go
--- a/uri.go
+++ b/uri.go
@@ -10,5 +10,8 @@
 func GUID() *url.URL {
+    return &url.URL{Scheme: "file", Opaque: "abc123"}
 }
`)
	got := len(SLP218{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for non-path opaque, got %d", got)
	}
}
