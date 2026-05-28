package rules

import (
	"strings"
	"testing"
)

func TestSLP058_FiresOnSQLConcat(t *testing.T) {
	d := parseDiff(t, `diff --git a/db.go b/db.go
--- a/db.go
+++ b/db.go
@@ -1,2 +1,3 @@
 package db
+
+query := "SELECT * FROM users WHERE id = " + userID
`)
	got := SLP058{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "SQL built with string concatenation") {
		t.Errorf("message: %q", got[0].Message)
	}
}

func TestSLP058_FiresOnFmtSprintf(t *testing.T) {
	d := parseDiff(t, `diff --git a/db.go b/db.go
--- a/db.go
+++ b/db.go
@@ -1,2 +1,3 @@
 package db
+
+query := fmt.Sprintf("SELECT * FROM users WHERE id = %s", id)
`)
	got := SLP058{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(got), got)
	}
}

func TestSLP058_IgnoresFmtSprintfWithoutFormatVerb(t *testing.T) {
	d := parseDiff(t, `diff --git a/db.go b/db.go
--- a/db.go
+++ b/db.go
@@ -1,2 +1,3 @@
 package db
+
+query := fmt.Sprintf("SELECT * FROM users")
`)
	got := SLP058{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for fmt.Sprintf without formatting verb, got %d: %+v", len(got), got)
	}
}

func TestSLP058_FiresOnInterpolation(t *testing.T) {
	d := parseDiff(t, `diff --git a/db.js b/db.js
--- a/db.js
+++ b/db.js
@@ -1,1 +1,2 @@
 const x = 1
+
+const q = "SELECT * FROM users WHERE id = ${userId}"
`)
	got := SLP058{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(got), got)
	}
}

func TestSLP058_IgnoresPythonDBPlaceholders(t *testing.T) {
	d := parseDiff(t, `diff --git a/db.py b/db.py
--- a/db.py
+++ b/db.py
@@ -1,2 +1,3 @@
 def get_user(id):
+
     cursor.execute("SELECT * FROM users WHERE id = %s", id)
`)
	got := SLP058{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for Python DB placeholder, got %d: %+v", len(got), got)
	}
}

func TestSLP058_FiresOnPythonSQLConcat(t *testing.T) {
	d := parseDiff(t, `diff --git a/db.py b/db.py
--- a/db.py
+++ b/db.py
@@ -1,2 +1,3 @@
 def get_user(id):
+
+    cursor.execute("SELECT * FROM users WHERE id = " + id)
`)
	got := SLP058{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for Python SQL concatenation, got %d: %+v", len(got), got)
	}
}

func TestSLP058_IgnoresPlainSQL(t *testing.T) {
	d := parseDiff(t, `diff --git a/db.go b/db.go
--- a/db.go
+++ b/db.go
@@ -1,2 +1,3 @@
 package db
+
+query := "SELECT * FROM users"
`)
	got := SLP058{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for plain SQL, got %d: %+v", len(got), got)
	}
}

func TestSLP058_IgnoresLowercaseProseInJSXDescription(t *testing.T) {
	// The 2026-05-28 false positive on whimsy BackupsPage.tsx:415:
	// `description="Backups run from a connector to a repository ..."` was
	// block-flagged because lowercase "from" + a "+" appearing later in the
	// JSX expression matched the old case-insensitive regex. SLP058 now
	// requires UPPERCASE SQL keywords by default (or a SQL execution context
	// nearby), so English prose passes silently.
	d := parseDiff(t, `diff --git a/app/BackupsPage.tsx b/app/BackupsPage.tsx
--- a/app/BackupsPage.tsx
+++ b/app/BackupsPage.tsx
@@ -1,5 +1,8 @@
 export function Empty() {
+  return (
+    <EmptyState
+      title="No backup policies"
+      description={"Backups run from a connector to a repository — set one up first, then "+ "the policy lands from the same screen."}
+    />
+  );
 }
`)
	got := SLP058{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings on JSX prose, got %d: %+v", len(got), got)
	}
}

func TestSLP058_IgnoresLowercaseProseInGenericTSFile(t *testing.T) {
	// Plain .ts file with English prose containing lowercase "from" must not
	// fire. Without a SQL execution context, lowercase keywords are treated
	// as prose.
	d := parseDiff(t, `diff --git a/app/copy.ts b/app/copy.ts
--- a/app/copy.ts
+++ b/app/copy.ts
@@ -1,2 +1,3 @@
 export const messages = {
+  empty: "Pick a row from the table — selection persists across reloads " + suffix,
 };
`)
	got := SLP058{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings on plain prose, got %d: %+v", len(got), got)
	}
}

func TestSLP058_FiresOnLowercaseSQLWithExecutionContext(t *testing.T) {
	// Lowercase SQL is still flagged when paired with a SQL execution call —
	// `pool.query("select ... from ... where ... " + var)` is a real
	// vulnerability regardless of casing.
	d := parseDiff(t, `diff --git a/db.js b/db.js
--- a/db.js
+++ b/db.js
@@ -1,2 +1,3 @@
 async function getUser(id) {
+  return pool.query("select * from users where id = " + id);
 }
`)
	got := SLP058{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding on lowercase SQL with pool.query, got %d: %+v", len(got), got)
	}
}

func TestSLP058_FiresOnUppercaseSQLInTSFile(t *testing.T) {
	// Regression guard: uppercase production SQL in a .ts file must still fire,
	// even without a recognizable SQL execution call on the line.
	d := parseDiff(t, `diff --git a/repo.ts b/repo.ts
--- a/repo.ts
+++ b/repo.ts
@@ -1,2 +1,3 @@
 async function get(id) {
+  const q = "SELECT * FROM users WHERE id = " + id;
 }
`)
	got := SLP058{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding on uppercase SQL, got %d: %+v", len(got), got)
	}
}

func TestSLP058_Description(t *testing.T) {
	r := SLP058{}
	if r.ID() != "SLP058" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityBlock {
		t.Errorf("default severity should be block")
	}
}
