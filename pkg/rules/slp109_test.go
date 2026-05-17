package rules

import "testing"

func TestSLP109_FiresOnDuplicateFunction(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -1,1 +1,18 @@
+func ProcessUser(id string) error {
+    ctx := context.Background()
+    validate(id)
+    log.Printf("processing %s", id)
+    result := db.Insert("users", id)
+    return result
+}
+
+func ProcessItem(id string) error {
+    ctx := context.Background()
+    validate(id)
+    log.Printf("processing %s", id)
+    result := db.Insert("items", id)
+    return result
+}
`)
	got := SLP109{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for duplicate function")
	}
}

func TestSLP109_NoFireOnDifferentFunctions(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -1,1 +1,10 @@
+func ProcessUser(id string) error {
+    return db.Insert("users", id)
+}
+
+func GetMetrics() []Metric {
+    metrics := fetchMetrics()
+    return metrics
+}
`)
	got := SLP109{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for different functions, got %d", len(got))
	}
}

func TestSLP109_FiresWhenBraceStartsOnNextLine(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -1,1 +1,22 @@
+func ProcessUser(id string) error
+{
+    ctx := context.Background()
+    validate(id)
+    log.Printf("processing %s", id)
+    result := db.Insert("users", id)
+    return result
+}
+
+func ProcessItem(id string) error
+{
+    ctx := context.Background()
+    validate(id)
+    log.Printf("processing %s", id)
+    result := db.Insert("items", id)
+    return result
+}
`)
	got := SLP109{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for duplicate multiline function signatures")
	}
}

func TestSLP109_FiresOnWrappedParameterList(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -1,1 +1,24 @@
+func ProcessUser(
+    id string,
+    mode string,
+) error {
+    validate(id)
+    log.Printf("processing %s", id)
+    return db.Insert("users", id)
+}
+
+func ProcessItem(
+    id string,
+    mode string,
+) error {
+    validate(id)
+    log.Printf("processing %s", id)
+    return db.Insert("items", id)
+}
`)
	got := SLP109{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for wrapped parameter list")
	}
}

func TestSLP109_EmitsAtMostOneFindingPerTargetFunction(t *testing.T) {
	d := parseDiff(t, `diff --git a/handlers.go b/handlers.go
--- a/handlers.go
+++ b/handlers.go
@@ -1,1 +1,26 @@
+func ProcessUser(id string) error {
+    validate(id)
+    log.Printf("processing %s", id)
+    return db.Insert("users", id)
+}
+
+func ProcessItem(id string) error {
+    validate(id)
+    log.Printf("processing %s", id)
+    return db.Insert("items", id)
+}
+
+func ProcessOrder(id string) error {
+    validate(id)
+    log.Printf("processing %s", id)
+    return db.Insert("orders", id)
+}
`)
	got := SLP109{}.Check(d)
	if len(got) != 2 {
		t.Fatalf("expected 2 findings for 3 similar functions, got %d", len(got))
	}
}

func TestSLP109_NoPanicOnIndentedMultiLineSignature(t *testing.T) {
	// A multi-line signature whose `) {` body-start line is indented:
	// braceOff is a raw-line offset, and slicing the trimmed copy used
	// to panic with slice-out-of-range.
	d := parseDiff(t, "diff --git a/h.go b/h.go\n"+
		"--- a/h.go\n"+
		"+++ b/h.go\n"+
		"@@ -1,1 +1,8 @@\n"+
		" package h\n"+
		"+func outer() {\n"+
		"+\tinner := func(\n"+
		"+\t\taValue int,\n"+
		"+\t) {\n"+
		"+\t\treturn aValue\n"+
		"+\t}\n"+
		"+\t_ = inner\n"+
		"+}\n")
	// Must not panic.
	_ = SLP109{}.Check(d)
}

func TestSLP109_Description(t *testing.T) {
	r := SLP109{}
	if r.ID() != "SLP109" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityWarn {
		t.Errorf("default severity should be warn")
	}
}
