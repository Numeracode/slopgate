package rules

import "testing"

func TestSLP152_FiresAfterTerminatingIfElse(t *testing.T) {
	d := parseDiff(t, `diff --git a/h.go b/h.go
--- a/h.go
+++ b/h.go
@@ -1,1 +1,9 @@
 package h
+func F(x bool) int {
+  if x {
+    return 1
+  } else {
+    return 2
+  }
+  cleanup()
+}
`)
	got := SLP152{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected an unreachable-code finding after a terminating if/else")
	}
}

func TestSLP152_FiresAfterTerminatingIfElseIfElse(t *testing.T) {
	d := parseDiff(t, `diff --git a/h.go b/h.go
--- a/h.go
+++ b/h.go
@@ -1,1 +1,11 @@
 package h
+func F(x int) int {
+  if x == 1 {
+    return 1
+  } else if x == 2 {
+    return 2
+  } else {
+    return 3
+  }
+  log("done")
+}
`)
	got := SLP152{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected a finding after a terminating if/else-if/else chain")
	}
}

func TestSLP152_NoFireWhenABranchDoesNotTerminate(t *testing.T) {
	d := parseDiff(t, `diff --git a/h.go b/h.go
--- a/h.go
+++ b/h.go
@@ -1,1 +1,9 @@
 package h
+func F(x bool) int {
+  if x {
+    return 1
+  } else {
+    doWork()
+  }
+  cleanup()
+}
`)
	got := SLP152{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when the else branch falls through, got %d", len(got))
	}
}

func TestSLP152_NoFireWithoutElse(t *testing.T) {
	d := parseDiff(t, `diff --git a/h.go b/h.go
--- a/h.go
+++ b/h.go
@@ -1,1 +1,7 @@
 package h
+func F(x bool) int {
+  if x {
+    return 1
+  }
+  cleanup()
+}
`)
	got := SLP152{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for an if without else, got %d", len(got))
	}
}

func TestSLP152_NoFireWhenBranchReturnIsNested(t *testing.T) {
	d := parseDiff(t, `diff --git a/h.go b/h.go
--- a/h.go
+++ b/h.go
@@ -1,1 +1,11 @@
 package h
+func F(a, b bool) int {
+  if a {
+    if b {
+      return 1
+    }
+  } else {
+    return 2
+  }
+  cleanup()
+}
`)
	got := SLP152{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings: the if branch only returns conditionally, got %d", len(got))
	}
}

func TestSLP152_FiresInJavaScript(t *testing.T) {
	d := parseDiff(t, `diff --git a/h.ts b/h.ts
--- a/h.ts
+++ b/h.ts
@@ -1,1 +1,9 @@
 const ready = true;
+function pick(x: boolean): number {
+  if (x) {
+    return 1;
+  } else {
+    throw new Error("no");
+  }
+  notReached();
+}
`)
	got := SLP152{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected an unreachable-code finding in a TypeScript if/else")
	}
}

func TestSLP152_Description(t *testing.T) {
	r := SLP152{}
	if r.ID() != "SLP152" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityWarn {
		t.Errorf("default severity should be warn")
	}
}
