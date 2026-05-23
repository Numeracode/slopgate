package rules

import (
	"testing"
)

func TestSLP068_FiresOnDuplicateBlock(t *testing.T) {
	d := parseDiff(t, `diff --git a/utils.go b/utils.go
--- a/utils.go
+++ b/utils.go
@@ -1,1 +1,19 @@
 package utils
+a := 1
+b := 2
+c := 3
+d := 4
+e := 5
+f := 6
+g := 7
+h := 8
+a := 1
+b := 2
+c := 3
+d := 4
+e := 5
+f := 6
+g := 7
+h := 8
`)
	got := SLP068{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(got), got)
	}
}

func TestSLP068_NoFireOnShortDuplicate(t *testing.T) {
	d := parseDiff(t, `diff --git a/utils.go b/utils.go
--- a/utils.go
+++ b/utils.go
@@ -1,1 +1,7 @@
 package utils
+a := 1
+b := 2
+c := 3
+a := 1
+b := 2
+c := 3
`)
	got := SLP068{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(got), got)
	}
}

func TestSLP068_IgnoresDuplicateDocsBlock(t *testing.T) {
	d := parseDiff(t, `diff --git a/docs/plan.md b/docs/plan.md
--- a/docs/plan.md
+++ b/docs/plan.md
@@ -1,1 +1,12 @@
 # Plan
+This paragraph is intentionally repeated in a long design note.
+It describes a migration sequence and acceptance criteria.
+It is prose, not duplicated production logic.
+It may exceed several lines in a Markdown document.
+It should not be surfaced as a code clone.
+This paragraph is intentionally repeated in a long design note.
+It describes a migration sequence and acceptance criteria.
+It is prose, not duplicated production logic.
+It may exceed several lines in a Markdown document.
+It should not be surfaced as a code clone.
`)
	got := SLP068{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for duplicated docs prose, got %d: %+v", len(got), got)
	}
}

func TestSLP068_IgnoresTestFiles(t *testing.T) {
	d := parseDiff(t, `diff --git a/api/tests/example.test.js b/api/tests/example.test.js
--- a/api/tests/example.test.js
+++ b/api/tests/example.test.js
@@ -1,1 +1,11 @@
 describe("x", () => {
+  const payload = { ok: true }
+  expect(payload.ok).toBe(true)
+  call(payload)
+  expect(call).toHaveBeenCalled()
+  cleanup()
+  const payload = { ok: true }
+  expect(payload.ok).toBe(true)
+  call(payload)
+  expect(call).toHaveBeenCalled()
+  cleanup()
 })
`)
	got := SLP068{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for duplicated test setup, got %d: %+v", len(got), got)
	}
}

func TestSLP068_CollapsesOverlappingWindowSpam(t *testing.T) {
	d := parseDiff(t, `diff --git a/component.tsx b/component.tsx
--- a/component.tsx
+++ b/component.tsx
@@ -1,1 +1,19 @@
 export function View() {
+  const a = one()
+  const b = two()
+  const c = three()
+  const d = four()
+  const e = five()
+  const f = six()
+  const g = seven()
+  const h = eight()
+  const a = one()
+  const b = two()
+  const c = three()
+  const d = four()
+  const e = five()
+  const f = six()
+  const g = seven()
+  const h = eight()
 }
`)
	got := SLP068{}.Check(d)
	if len(got) != 1 {
		t.Fatalf("expected 1 collapsed finding for duplicate code block, got %d: %+v", len(got), got)
	}
}

func TestSLP068_IgnoresGeneratedArtifacts(t *testing.T) {
	d := parseDiff(t, `diff --git a/packages/sdk/src/generated/operations.ts b/packages/sdk/src/generated/operations.ts
--- a/packages/sdk/src/generated/operations.ts
+++ b/packages/sdk/src/generated/operations.ts
@@ -1,1 +1,19 @@
 export const operations = {}
+  const a = one()
+  const b = two()
+  const c = three()
+  const d = four()
+  const e = five()
+  const f = six()
+  const g = seven()
+  const h = eight()
+  const a = one()
+  const b = two()
+  const c = three()
+  const d = four()
+  const e = five()
+  const f = six()
+  const g = seven()
+  const h = eight()
 `)
	got := SLP068{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for generated artifact, got %d: %+v", len(got), got)
	}
}

func TestSLP068_IgnoresOpenAPIArtifacts(t *testing.T) {
	d := parseDiff(t, `diff --git a/openapi/spec.yaml b/openapi/spec.yaml
--- a/openapi/spec.yaml
+++ b/openapi/spec.yaml
@@ -1,1 +1,19 @@
 openapi: 3.1.0
+  const a = one()
+  const b = two()
+  const c = three()
+  const d = four()
+  const e = five()
+  const f = six()
+  const g = seven()
+  const h = eight()
+  const a = one()
+  const b = two()
+  const c = three()
+  const d = four()
+  const e = five()
+  const f = six()
+  const g = seven()
+  const h = eight()
 `)
	got := SLP068{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for OpenAPI artifact, got %d: %+v", len(got), got)
	}
}

func TestSLP068_Meta(t *testing.T) {
	r := SLP068{}
	if r.ID() != "SLP068" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityInfo {
		t.Errorf("default severity should be info")
	}
}
