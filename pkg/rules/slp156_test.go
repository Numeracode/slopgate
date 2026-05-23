package rules

import "testing"

func TestSLP156_FiresOnNullOrUndefined(t *testing.T) {
	cases := []string{
		`+    const val = x === null || x === undefined ? undefined : use(x);`,
		`+    if (foo.bar === undefined || foo.bar === null) { return; }`,
		`+    return value === null || value === undefined;`,
	}
	for _, c := range cases {
		d := parseDiff(t, "diff --git a/src/util.ts b/src/util.ts\n--- a/src/util.ts\n+++ b/src/util.ts\n@@ -1 +1,2 @@\n"+c+"\n")
		got := SLP156{}.Check(d)
		if len(got) == 0 {
			t.Errorf("expected finding for: %s", c)
		}
	}
}

func TestSLP156_FiresOnNotNullAndNotUndefined(t *testing.T) {
	cases := []string{
		`+    if (x !== null && x !== undefined) { process(x); }`,
		`+    const ok = val !== undefined && val !== null;`,
	}
	for _, c := range cases {
		d := parseDiff(t, "diff --git a/src/util.js b/src/util.js\n--- a/src/util.js\n+++ b/src/util.js\n@@ -1 +1,2 @@\n"+c+"\n")
		got := SLP156{}.Check(d)
		if len(got) == 0 {
			t.Errorf("expected finding for: %s", c)
		}
	}
}

func TestSLP156_NoFireWhenDifferentVariables(t *testing.T) {
	d := parseDiff(t, `diff --git a/src/util.ts b/src/util.ts
--- a/src/util.ts
+++ b/src/util.ts
@@ -1 +1,2 @@
+    if (a === null || b === undefined) { return; }
`)
	got := SLP156{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for different variables, got %d", len(got))
	}
}

func TestSLP156_NoFireOnIdiomatic(t *testing.T) {
	cases := []string{
		`+    if (x == null) { return; }`,
		`+    const v = x ?? defaultValue;`,
		`+    if (x != null) { use(x); }`,
	}
	for _, c := range cases {
		d := parseDiff(t, "diff --git a/src/util.ts b/src/util.ts\n--- a/src/util.ts\n+++ b/src/util.ts\n@@ -1 +1,2 @@\n "+c+"\n")
		got := SLP156{}.Check(d)
		if len(got) != 0 {
			t.Errorf("expected 0 findings for idiomatic pattern %q, got %d", c, len(got))
		}
	}
}

func TestSLP156_NoFireOnNonJSFile(t *testing.T) {
	d := parseDiff(t, `diff --git a/pkg/util.go b/pkg/util.go
--- a/pkg/util.go
+++ b/pkg/util.go
@@ -1 +1,2 @@
+    // x === null || x === undefined is a JS pattern, not Go
`)
	got := SLP156{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for Go file, got %d", len(got))
	}
}

// Member-access chains (e.g. verification.snapshotRecordId) — the exact pattern
// from the whimsy benchmark miss.
func TestSLP156_FiresOnMemberAccess(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/src/lib/connectors/wsServer.js b/api/src/lib/connectors/wsServer.js
--- a/api/src/lib/connectors/wsServer.js
+++ b/api/src/lib/connectors/wsServer.js
@@ -258,0 +259,3 @@
+        snapshotRecordId: verification.snapshotRecordId === undefined || verification.snapshotRecordId === null
+          ? undefined
+          : assertUuid(verification.snapshotRecordId, 'verification.snapshotRecordId'),
`)
	got := SLP156{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected finding for member-access double null-check (wsServer.js pattern)")
	}
}

func TestSLP156_Description(t *testing.T) {
	r := SLP156{}
	if r.ID() != "SLP156" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityInfo {
		t.Errorf("default severity = %v, want info", r.DefaultSeverity())
	}
}
