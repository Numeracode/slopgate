package rules

import "testing"

func TestSLP151_FiresOnTestCallingRemovedFunction(t *testing.T) {
	d := parseDiff(t, `diff --git a/user.go b/user.go
--- a/user.go
+++ b/user.go
@@ -1,5 +1,1 @@
 package user
-
-func GetUser(id int) error {
-	return nil
-}
diff --git a/user_test.go b/user_test.go
--- a/user_test.go
+++ b/user_test.go
@@ -1,1 +1,3 @@
 package user
+
+func TestGetUser(t *testing.T) { GetUser(1) }
`)
	got := SLP151{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected an orphaned-test finding for a test calling a removed function")
	}
}

func TestSLP151_NoFireWhenFunctionModifiedInPlace(t *testing.T) {
	d := parseDiff(t, `diff --git a/user.go b/user.go
--- a/user.go
+++ b/user.go
@@ -1,3 +1,3 @@
 package user
-func GetUser(id int) error {
+func GetUser(id int64) error {
 	return nil
diff --git a/user_test.go b/user_test.go
--- a/user_test.go
+++ b/user_test.go
@@ -1,1 +1,2 @@
 package user
+func TestGetUser(t *testing.T) { GetUser(1) }
`)
	got := SLP151{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when the function was edited in place, got %d", len(got))
	}
}

func TestSLP151_NoFireWhenFunctionMovedToAnotherFile(t *testing.T) {
	d := parseDiff(t, `diff --git a/old.go b/old.go
--- a/old.go
+++ b/old.go
@@ -1,2 +1,1 @@
 package user
-func GetUser(id int) error { return nil }
diff --git a/new.go b/new.go
--- a/new.go
+++ b/new.go
@@ -1,1 +1,2 @@
 package user
+func GetUser(id int) error { return nil }
diff --git a/user_test.go b/user_test.go
--- a/user_test.go
+++ b/user_test.go
@@ -1,1 +1,2 @@
 package user
+func TestGetUser(t *testing.T) { GetUser(1) }
`)
	got := SLP151{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings when the function was moved, not removed, got %d", len(got))
	}
}

func TestSLP151_NoFireOnLocallyDefinedSymbol(t *testing.T) {
	d := parseDiff(t, `diff --git a/svc.go b/svc.go
--- a/svc.go
+++ b/svc.go
@@ -1,2 +1,1 @@
 package svc
-func process(x int) int { return x }
diff --git a/svc_test.go b/svc_test.go
--- a/svc_test.go
+++ b/svc_test.go
@@ -1,1 +1,3 @@
 package svc
+func process(x int) int { return x * 2 }
+func TestProcess(t *testing.T) { process(2) }
`)
	got := SLP151{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings: the test defines its own process(), got %d", len(got))
	}
}

func TestSLP151_FiresOnJSRemovedExport(t *testing.T) {
	d := parseDiff(t, `diff --git a/api.ts b/api.ts
--- a/api.ts
+++ b/api.ts
@@ -1,2 +1,1 @@
 const base = 1;
-export function fetchUser(id) { return null; }
diff --git a/api.test.ts b/api.test.ts
--- a/api.test.ts
+++ b/api.test.ts
@@ -1,1 +1,2 @@
 const ready = true;
+test("fetchUser", () => { fetchUser(1); });
`)
	got := SLP151{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected an orphaned-test finding for a JS test calling a removed export")
	}
}

func TestSLP151_NoFireOnShortSymbolName(t *testing.T) {
	d := parseDiff(t, `diff --git a/m.go b/m.go
--- a/m.go
+++ b/m.go
@@ -1,2 +1,1 @@
 package m
-func id(x int) int { return x }
diff --git a/m_test.go b/m_test.go
--- a/m_test.go
+++ b/m_test.go
@@ -1,1 +1,2 @@
 package m
+func TestID(t *testing.T) { id(1) }
`)
	got := SLP151{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for a sub-3-char symbol name, got %d", len(got))
	}
}

func TestSLP151_FiresOnRemovedTSMethod(t *testing.T) {
	d := parseDiff(t, `diff --git a/svc.ts b/svc.ts
--- a/svc.ts
+++ b/svc.ts
@@ -1,5 +1,2 @@
 class Service {
-  fetchAll(): string[] {
-    return [];
-  }
 }
diff --git a/svc.test.ts b/svc.test.ts
--- a/svc.test.ts
+++ b/svc.test.ts
@@ -1,1 +1,2 @@
 const s = new Service();
+test("fetchAll", () => { s.fetchAll(); });
`)
	got := SLP151{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected an orphaned-test finding for a test calling a removed TS method")
	}
}

func TestSLP151_NoFireOnControlFlowKeywords(t *testing.T) {
	// A removed `for (...) {` must not register `for` as a definition.
	d := parseDiff(t, `diff --git a/loop.ts b/loop.ts
--- a/loop.ts
+++ b/loop.ts
@@ -1,4 +1,1 @@
 const xs = [1];
-for (const x of xs) {
-  use(x);
-}
diff --git a/loop.test.ts b/loop.test.ts
--- a/loop.test.ts
+++ b/loop.test.ts
@@ -1,1 +1,2 @@
 const ready = true;
+test("loop", () => { for (const y of [1]) {} });
`)
	got := SLP151{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings: control-flow keywords are not definitions, got %d", len(got))
	}
}

func TestSLP151_Description(t *testing.T) {
	r := SLP151{}
	if r.ID() != "SLP151" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityWarn {
		t.Errorf("default severity should be warn")
	}
}
