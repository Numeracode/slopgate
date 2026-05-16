package rules

import "testing"

func TestSLP118_FiresOnDirectIndexAccess(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,3 @@
 package main
+
+var first = items[0]
`)
	got := SLP118{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for index access without guard")
	}
}

func TestSLP118_FiresOnDirectIndexAccessBeyondOne(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,3 @@
 package main
+
+var third = items[2]
`)
	got := SLP118{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for index [2] access without guard")
	}
}

func TestSLP118_NoFireOnSlicing(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,3 @@
 package main
+
+var subset = items[1:3]
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for slicing, got %d", len(got))
	}
}

func TestSLP118_NoFireOnGuardedAccess(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,4 @@
 package main
+
+if len(items) > 0 {
+    var first = items[0]
+}
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for guarded access, got %d", len(got))
	}
}

func TestSLP118_FireOnUnguardedDifferentCollection(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,4 @@
 package main
+
+if len(items) > 0 {
+    var first = other[0]
+}
`)
	got := SLP118{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for unguarded different collection index access")
	}
}

func TestSLP118_NoFireOnJSGuardedAccess(t *testing.T) {
	d := parseDiff(t, `diff --git a/app.ts b/app.ts
--- a/app.ts
+++ b/app.ts
@@ -1,1 +1,4 @@
 const x = 1
+
+if (items.length > 0) {
+    const first = items[0]
+}
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for JS guarded access, got %d", len(got))
	}
}

func TestSLP118_NoFireOnPyGuardedAccess(t *testing.T) {
	d := parseDiff(t, `diff --git a/app.py b/app.py
--- a/app.py
+++ b/app.py
@@ -1,1 +1,4 @@
 x = 1
+
+if len(items) > 0:
+    first = items[0]
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for Python guarded access, got %d", len(got))
	}
}

func TestSLP118_PrevContentPreservedAcrossContextLines(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,3 +1,5 @@
 package main
-// old line
+if len(items) > 0 {
 // safe access
+    var first = items[0]
+}
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when guard is present via context line, got %d", len(got))
	}
}

func TestSLP118_NoFireOnArrayTypeDeclaration(t *testing.T) {
	d := parseDiff(t, `diff --git a/types.go b/types.go
--- a/types.go
+++ b/types.go
@@ -1,1 +1,3 @@
 package main
+
+var buf [16]byte
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for array type declaration, got %d", len(got))
	}
}

func TestSLP118_FiresOnChainedIndexAfterCall(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,3 @@
 package main
+
+var result = getData()[0]
`)
	got := SLP118{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for chained index access without guard")
	}
}

func TestSLP118_CompoundGuardCoversBothCollections(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,4 @@
 package main
+
+if len(a) > 0 && len(b) > 0 {
+    x = a[0]
+}
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for compound guard covering a[0], got %d", len(got))
	}
}

func TestSLP118_CompoundGuardDoesNotCoverOtherCollection(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,4 @@
 package main
+
+if len(a) > 0 && len(b) > 0 {
+    x = c[0]
+}
`)
	got := SLP118{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for unguarded c[0] with compound guard on a,b")
	}
}

func TestSLP118_NoFireOnGuardedSubmatchAccess(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,6 @@
 package main

+if len(match) >= 3 {
+    key := match[1]
+    value := match[2]
+    use(key, value)
+}
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for guarded submatch access, got %d: %+v", len(got), got)
	}
}

func TestSLP118_InlineAccessOnGuardLine(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,3 @@
 package main
+
+if len(items) > 0 { use(other[0]) }
`)
	got := SLP118{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for other[0] on same line as guard for items")
	}
}

func TestSLP118_ContextLineGuardHonored(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,2 +1,3 @@
 if len(items) > 0 {
-    x = items[0]
+    y = items[0]
 }
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when guard is on context line, got %d", len(got))
	}
}

func TestSLP118_NoFireAfterEarlyExitGuard(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,6 @@
 package main
+
+items := collect()
+if len(items) == 0 {
+    return
+}
+var first = items[0]
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings after early-exit len==0 guard, got %d", len(got))
	}
}

func TestSLP118_NoFireAfterEarlyExitGuardContinue(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,7 @@
 package main
+
+for _, group := range groups {
+    lines := group.Added()
+    if len(lines) == 0 {
+        continue
+    }
+    use(lines[0])
+}
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings after early-exit len==0 continue guard, got %d", len(got))
	}
}

func TestSLP118_NoFireAfterEarlyExitGuardJS(t *testing.T) {
	d := parseDiff(t, `diff --git a/app.ts b/app.ts
--- a/app.ts
+++ b/app.ts
@@ -1,1 +1,5 @@
 const x = 1
+
+if (rows.length === 0) {
+  return null
+}
+const head = rows[0]
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings after JS early-exit length===0 guard, got %d", len(got))
	}
}

func TestSLP118_FireWhenEmptyCheckHasNoExit(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,5 @@
 package main
+
+if len(items) == 0 {
+    items = defaults()
+}
+var first = items[0]
`)
	got := SLP118{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected a finding when len==0 check does not early-exit")
	}
}

func TestSLP118_EarlyExitGuardScopedToBlock(t *testing.T) {
	d := parseDiff(t, `diff --git a/process.go b/process.go
--- a/process.go
+++ b/process.go
@@ -1,1 +1,11 @@
 package main
+
+func first() {
+	items := load()
+	if len(items) == 0 {
+		return
+	}
+}
+func second() {
+	items := load()
+	head := items[0]
+}
`)
	got := SLP118{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected a finding: early-exit guard in first() must not cover items[0] in second()")
	}
}

func TestSLP118_EarlyExitGuardSpansHunkBoundary(t *testing.T) {
	d := parseDiff(t, `diff --git a/svc.go b/svc.go
--- a/svc.go
+++ b/svc.go
@@ -3,3 +3,6 @@
 func process() {
 	items := load()
+	if len(items) == 0 {
+		return
+	}
 	log("have items")
@@ -18,2 +21,3 @@
 	prepare()
+	first := items[0]
 	use(first)
`)
	got := SLP118{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings: guard in hunk 1 should cover items[0] in hunk 2, got %d", len(got))
	}
}

func TestSLP118_Description(t *testing.T) {
	r := SLP118{}
	if r.ID() != "SLP118" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityBlock {
		t.Errorf("default severity should be block")
	}
}
