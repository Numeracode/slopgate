package rules

import "testing"

func TestSLP013_FiresOnThreeCommentedGoStatements(t *testing.T) {
	d := parseDiff(t, `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,3 +1,7 @@
 package a

+// old := foo()
+// bar(old, 42)
+// return old + 1
 func New() {}
 var x = 1
`)
	got := SLP013{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for the block, got %d: %+v", len(got), got)
	}
}

func TestSLP013_IgnoresProseComments(t *testing.T) {
	// A block of ordinary prose comments (sentences, no code shapes)
	// should NOT fire. This is the false-positive guard.
	d := parseDiff(t, `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,3 +1,7 @@
 package a

+// This helper walks the tree in reverse order, because we want to
+// surface the newest items first and the caller already sorted them.
+// The implementation uses a plain for loop to avoid allocating a stack.
 func Walk() {}
 var x = 1
`)
	got := SLP013{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for prose, got %d: %+v", len(got), got)
	}
}

func TestSLP013_IgnoresShortCommentBlocks(t *testing.T) {
	// Two lines is not enough to flag — commenting out two lines is often
	// a legitimate diff review artifact.
	d := parseDiff(t, `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,3 @@
 package a
+// old := foo()
+// bar(old)
`)
	got := SLP013{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for 2-line block, got %d", len(got))
	}
}

func TestSLP013_FiresOnPythonCommentedCode(t *testing.T) {
	d := parseDiff(t, `diff --git a/a.py b/a.py
--- a/a.py
+++ b/a.py
@@ -1,1 +1,5 @@
 import sys
+# result = calculate(x, y)
+# if result > threshold:
+#     send_alert(result)
+# return result
`)
	got := SLP013{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
}

func TestSLP013_IgnoresDocFiles(t *testing.T) {
	// In markdown, prefixing lines with `//` is an odd but legitimate
	// fenced-code-block choice. Don't fire in docs.
	d := parseDiff(t, `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1,1 +1,5 @@
 # Docs
+// const foo = 1;
+// const bar = 2;
+// const baz = foo + bar;
+// console.log(baz);
`)
	got := SLP013{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings in README.md, got %d", len(got))
	}
}

func TestSLP013_IgnoresGodocWithParens(t *testing.T) {
	// Real false positive from codero: godoc comment with a parenthetical
	// example. The paren contents are prose, not code.
	d := parseDiff(t, `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,4 @@
 package a
+// agentKind is the CLI family string (e.g. "claude", "gemini") stored in
+// the sessions.agent_id column. agentID is the Codero profile ID,
+// distinct from agentKind.
`)
	got := SLP013{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings on godoc prose with parens, got %d: %+v", len(got), got)
	}
}

func TestSLP013_IgnoresGodocWithReturns(t *testing.T) {
	// Real false positive: "returns" was matching the "return " token.
	d := parseDiff(t, `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,4 @@
 package a
+// queryActiveSessionForRepoBranch returns the tmux session name,
+// family/agent_id, and session ID for the currently active session
+// on the given repo and branch.
`)
	got := SLP013{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings on godoc with 'returns', got %d: %+v", len(got), got)
	}
}

func TestSLP013_IgnoresMigrationNotes(t *testing.T) {
	// Real false positive: prose notes about a React migration that happen
	// to contain JSX-shaped characters.
	d := parseDiff(t, `diff --git a/x.js b/x.js
--- a/x.js
+++ b/x.js
@@ -1,1 +1,4 @@
 import X from 'y';
+// REACT MIGRATION: mountEditor() is replaced by a <MonacoPane ref={paneRef}> component.
+// The ref forwards to the underlying Monaco instance so imperative calls still work.
+// See docs/frontend/migration.md for the full plan.
`)
	got := SLP013{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings on prose migration notes, got %d: %+v", len(got), got)
	}
}

func TestSLP013_IgnoresConfigFiles(t *testing.T) {
	// YAML/TOML/JSON config files are out of scope. Multi-line string
	// values in YAML heredocs can look like commented-out code to a
	// regex-level scanner, and commented-out *config* is a different
	// concern than commented-out *code* anyway.
	d := parseDiff(t, `diff --git a/.github/workflows/ci.yml b/.github/workflows/ci.yml
--- a/.github/workflows/ci.yml
+++ b/.github/workflows/ci.yml
@@ -1,1 +1,5 @@
 name: ci
+      body: |
+        ## Some Header
+        **field1:** value
+        **field2:** ${var}
`)
	got := SLP013{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings on YAML heredoc, got %d: %+v", len(got), got)
	}
}

func TestSLP013_IgnoresCSSVariableDeclarations(t *testing.T) {
	// Real false positive: CSS custom properties start with -- which
	// looked like a Lua/SQL comment prefix to an earlier version of
	// the rule.
	d := parseDiff(t, `diff --git a/styles.css b/styles.css
--- a/styles.css
+++ b/styles.css
@@ -1,1 +1,5 @@
 :root {
+  --bg-base: #0a0a0f;
+  --bg-surface-1: #12121a;
+  --bg-surface-2: #1a1a26;
+  --fg-primary: #f0f0f8;
`)
	got := SLP013{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings on CSS variables, got %d: %+v", len(got), got)
	}
}

func TestSLP013_IgnoresBulletListedCodeExamples(t *testing.T) {
	// Real false positive: a doc comment that bullet-lists example
	// API signatures looks like function calls but is documentation.
	d := parseDiff(t, `diff --git a/api.js b/api.js
--- a/api.js
+++ b/api.js
@@ -1,1 +1,5 @@
 export function foo() {}
+// Support both calling conventions:
+// - duplicateProfile(sourceId, newProfileId)
+// - duplicateProfile(sourceId, { new_profile_id: newId })
+// The first form is the legacy one and still works.
`)
	got := SLP013{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings on bullet-list examples, got %d: %+v", len(got), got)
	}
}

func TestSLP013_IgnoresProseQuotingCodeInBackticks(t *testing.T) {
	// A prose comment that quotes operators and calls in backticks is
	// documentation, not commented-out code.
	d := parseDiff(t, `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,5 @@
 package a
+// An arrow function with a `+"`=> expr`"+` body is judged at once,
+// while a `+"`=> {`"+` block body and a `+"`handler()`"+` call fall
+// through to the `+"`scanBody()`"+` pass described below.
 func F() {}
`)
	got := SLP013{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for prose quoting code in backticks, got %d: %+v", len(got), got)
	}
}

func TestSLP013_FiresOnCommentedGoRawStringStatement(t *testing.T) {
	// A genuinely commented-out Go statement keeps its assignment shape
	// outside the backticks and must still be flagged.
	d := parseDiff(t, "diff --git a/a.go b/a.go\n"+
		"--- a/a.go\n"+
		"+++ b/a.go\n"+
		"@@ -1,1 +1,5 @@\n"+
		" package a\n"+
		"+// q := `SELECT * FROM users`\n"+
		"+// rows := db.Query(q)\n"+
		"+// return rows.Err()\n"+
		" func F() {}\n")
	got := SLP013{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected a finding for commented-out Go statements that use a raw string")
	}
}

func TestSLP013_Description(t *testing.T) {
	r := SLP013{}
	if r.ID() != "SLP013" {
		t.Errorf("ID = %q", r.ID())
	}
}
