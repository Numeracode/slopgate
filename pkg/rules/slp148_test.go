package rules

import (
	"strings"
	"testing"
)

func TestSLP148_FiresOnInconsistentNaming(t *testing.T) {
	d := parseDiff(t, `diff --git a/service1.js b/service1.js
--- a/service1.js
+++ b/service1.js
@@ -1,2 +1,3 @@
 module.exports = {};
+export const userId = user.id;
diff --git a/service2.js b/service2.js
--- a/service2.js
+++ b/service2.js
@@ -1,2 +1,3 @@
 module.exports = {};
+export const userID = formatUser(req.user);
 `)
	got := SLP148{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding (userId vs userID), got %d: %+v", len(got), got)
	}
}

func TestSLP148_AllowsConsistentNaming(t *testing.T) {
	d := parseDiff(t, `diff --git a/service1.js b/service1.js
--- a/service1.js
+++ b/service1.js
@@ -1,2 +1,3 @@
 module.exports = {};
+export const userId = user.id;
diff --git a/service2.js b/service2.js
--- a/service2.js
+++ b/service2.js
@@ -1,2 +1,3 @@
 module.exports = {};
+export const userId = formatUser(req.user);
 `)
	got := SLP148{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings (consistent userId), got %d", len(got))
	}
}

func TestSLP148_AllowsDistinctFieldsWithSharedPrefix(t *testing.T) {
	d := parseDiff(t, `diff --git a/model.js b/model.js
--- a/model.js
+++ b/model.js
@@ -1,2 +1,4 @@
 module.exports = {};
+export const userId = user.id;
+export const userEmail = user.email;
 `)
	got := SLP148{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings (userId and userEmail are distinct fields), got %d", len(got))
	}
}

func TestSLP148_FiresOnSnakeVsCamelCase(t *testing.T) {
	d := parseDiff(t, `diff --git a/service1.js b/service1.js
--- a/service1.js
+++ b/service1.js
@@ -1,2 +1,3 @@
 module.exports = {};
+export const user_id = user.id;
diff --git a/service2.js b/service2.js
--- a/service2.js
+++ b/service2.js
@@ -1,2 +1,3 @@
 module.exports = {};
+export const userId = formatUser(req.user);
 `)
	got := SLP148{}.Check(d)
	if len(got) < 1 {
		t.Fatalf("expected >= 1 finding (user_id vs userId), got %d: %+v", len(got), got)
	}
}

func TestSLP148_FindingMessageContainsVariants(t *testing.T) {
	d := parseDiff(t, `diff --git a/a.js b/a.js
--- a/a.js
+++ b/a.js
@@ -1,2 +1,3 @@
 module.exports = {};
+export const userId = user.id;
diff --git a/b.js b/b.js
--- a/b.js
+++ b/b.js
@@ -1,2 +1,3 @@
 module.exports = {};
+export const userID = formatUser(req.user);
 `)
	got := SLP148{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(got), got)
	}
	if !containsAll(got[0].Message, "userId", "userID") {
		t.Fatalf("expected message to contain both variants, got: %s", got[0].Message)
	}
	if got[0].RuleID != "SLP148" {
		t.Fatalf("expected RuleID SLP148, got %s", got[0].RuleID)
	}
}

func TestSLP148_IgnoresIdentifiersInStrings(t *testing.T) {
	d := parseDiff(t, `diff --git a/a.js b/a.js
--- a/a.js
+++ b/a.js
@@ -1,2 +1,3 @@
 module.exports = {};
+export const label = "userId is great";
diff --git a/b.js b/b.js
--- a/b.js
+++ b/b.js
@@ -1,2 +1,3 @@
 module.exports = {};
+export const userID = formatUser(req.user);
 `)
	got := SLP148{}.Check(d)
	// "userId" inside a string literal should not be extracted
	if len(got) != 0 {
		t.Fatalf("expected 0 findings (userId in string literal), got %d: %+v", len(got), got)
	}
}

func TestSLP148_NilDiffReturnsNoFindings(t *testing.T) {
	got := SLP148{}.Check(nil)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for nil diff, got %d", len(got))
	}
}

func TestSLP148_IgnoresNonExportedLocals(t *testing.T) {
	// Non-exported locals: a casing inconsistency here is noise, not a
	// module-boundary problem, so SLP148 must stay silent.
	d := parseDiff(t, `diff --git a/a.js b/a.js
--- a/a.js
+++ b/a.js
@@ -1,1 +1,2 @@
 const ready = true;
+const userId = user.id;
diff --git a/b.js b/b.js
--- a/b.js
+++ b/b.js
@@ -1,1 +1,2 @@
 const ready = true;
+const userID = fetch();
`)
	got := SLP148{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for non-exported locals, got %d: %+v", len(got), got)
	}
}

func TestSLP148_FiresOnExportedGoDeclarations(t *testing.T) {
	d := parseDiff(t, `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,2 @@
 package a
+var UserId = 1
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1,1 +1,2 @@
 package b
+var UserID = 2
`)
	got := SLP148{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected a finding for inconsistent exported Go declarations (UserId vs UserID)")
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
