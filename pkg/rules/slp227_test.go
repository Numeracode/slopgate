package rules

import (
	"testing"
)

func TestSLP227RepeatedLiteral(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,12 @@
 func run() string {
+    a := "pending"
+    b := "pending"
+    c := "pending"
     return a + b + c
 }
`)
	got := len(SLP227{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}
}

func TestSLP227TrivialLiteralNotFlagged(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,10 @@
 func run() string {
+    a := ""
+    b := ""
+    c := ""
     return a + b + c
 }
`)
	got := len(SLP227{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for empty string, got %d", got)
	}
}

func TestSLP227FormatVerbNotFlagged(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,10 @@
 func run() string {
+    a := fmt.Sprintf("%s", "a")
+    b := fmt.Sprintf("%s", "b")
+    c := fmt.Sprintf("%s", "c")
     return a + b + c
 }
`)
	got := len(SLP227{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for format verb, got %d", got)
	}
}

func TestSLP227TwoCopiesOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,10 @@
 func run() string {
+    a := "pending"
+    b := "pending"
     return a + b
 }
`)
	got := len(SLP227{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for 2 copies, got %d", got)
	}
}

func TestSLP227SkipsOpenAPIFile(t *testing.T) {
	d := parseDiff(t, `diff --git a/docs/openapi/api.openapi.json b/docs/openapi/api.openapi.json
--- a/docs/openapi/api.openapi.json
+++ b/docs/openapi/api.openapi.json
@@ -10,5 +10,9 @@
+    "name": "test",
+    "name": "test",
+    "name": "test",
     "type": "string"
 }
`)
	got := len(SLP227{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for OpenAPI file, got %d", got)
	}
}

func TestSLP227SkipsTestFile(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker_test.go b/worker_test.go
--- a/worker_test.go
+++ b/worker_test.go
@@ -10,5 +10,9 @@
+    msg := "pending"
+    msg2 := "pending"
+    msg3 := "pending"
     t.Log(msg)
 }
`)
	got := len(SLP227{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for test file, got %d", got)
	}
}

func TestSLP227SkipsGeneratedFile(t *testing.T) {
	d := parseDiff(t, `diff --git a/pkg/generated/client.go b/pkg/generated/client.go
--- a/pkg/generated/client.go
+++ b/pkg/generated/client.go
@@ -10,5 +10,9 @@
+    key := "value"
+    key2 := "value"
+    key3 := "value"
     return key
 }
`)
	got := len(SLP227{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for generated file, got %d", got)
	}
}
