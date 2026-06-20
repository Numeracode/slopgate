package rules

import (
	"testing"
)

func TestSLP225GoroutineMapWrite(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,10 @@
 var resultCache = map[string]string{}
+
 func run(key, value string) {
+    go func() {
+        resultCache[key] = value
+    }()
 }
`)
	got := len(SLP225{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}
}

func TestSLP225GoroutineFieldWrite(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,10 @@
 var shared Result
+
 func run(r Result) {
+    go func() {
+        shared.total = r.total
+    }()
 }
`)
	got := len(SLP225{}.Check(d))
	if got != 1 {
		t.Fatalf("expected 1 finding, got %d", got)
	}
}

func TestSLP225GoroutineWithMutexOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,14 @@
 var mu sync.Mutex
 var resultCache = map[string]string{}
+
 func run(key, value string) {
+    go func() {
+        mu.Lock()
+        defer mu.Unlock()
+        resultCache[key] = value
+    }()
 }
`)
	got := len(SLP225{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings with mutex guard, got %d", got)
	}
}

func TestSLP225GoroutineAtomicOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,10 @@
 var counter int64
+
 func run() {
+    go func() {
+        atomic.AddInt64(&counter, 1)
+    }()
 }
`)
	got := len(SLP225{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings with atomic guard, got %d", got)
	}
}

func TestSLP225GoroutineNoWriteOK(t *testing.T) {
	d := parseDiff(t, `diff --git a/worker.go b/worker.go
--- a/worker.go
+++ b/worker.go
@@ -10,5 +10,10 @@
 func run() string {
+    go func() {
+        log.Println("background work")
+    }()
     return "ok"
 }
`)
	got := len(SLP225{}.Check(d))
	if got != 0 {
		t.Fatalf("expected 0 findings for goroutine without writes, got %d", got)
	}
}
