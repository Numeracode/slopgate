package rules

import "testing"

func TestSLP100_FiresOnNoOpFunction(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.go b/handler.go
--- a/handler.go
+++ b/handler.go
@@ -1,1 +1,5 @@
+func GetItems() ([]Item, error) {
+    return nil
+}
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for no-op function")
	}
}

func TestSLP100_FiresOnJavascriptNoOp(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.js b/handler.js
--- a/handler.js
+++ b/handler.js
@@ -1,1 +1,4 @@
+function getItems() {
+    return [];
+}
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for JS no-op")
	}
}

func TestSLP100_NoFireOnFunctionWithWork(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.go b/handler.go
--- a/handler.go
+++ b/handler.go
@@ -1,1 +1,5 @@
+func GetItems() ([]Item, error) {
+    items := db.Query("SELECT * FROM items")
+    return items, nil
+}
`)
	got := SLP100{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for function with work, got %d", len(got))
	}
}

func TestSLP100_NoFireOnNonEmptyStringReturn(t *testing.T) {
	d := parseDiff(t, `diff --git a/rule.go b/rule.go
--- a/rule.go
+++ b/rule.go
@@ -1,1 +1,5 @@
+func (Rule) Description() string {
+    return "rule description"
+}
`)
	got := SLP100{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for non-empty string return, got %d: %+v", len(got), got)
	}
}

func TestSLP100_NoFireOnNonEmptyStringReturnWithTrailingComment(t *testing.T) {
	d := parseDiff(t, `diff --git a/rule.go b/rule.go
--- a/rule.go
+++ b/rule.go
@@ -1,1 +1,5 @@
+func (Rule) Description() string {
+    return "rule description" // documented rule metadata
+}
`)
	got := SLP100{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for non-empty string return with comment, got %d: %+v", len(got), got)
	}
}

func TestSLP100_NoFireOnNonEmptyStringReturnWithTrailingBlockComment(t *testing.T) {
	d := parseDiff(t, `diff --git a/rule.go b/rule.go
--- a/rule.go
+++ b/rule.go
@@ -1,1 +1,5 @@
+func (Rule) Description() string {
+    return "rule description" /* documented rule metadata */
+}
`)
	got := SLP100{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for non-empty string return with block comment, got %d: %+v", len(got), got)
	}
}

func TestSLP100_FiresOnEmptyStringReturnWithTrailingComment(t *testing.T) {
	d := parseDiff(t, `diff --git a/rule.go b/rule.go
--- a/rule.go
+++ b/rule.go
@@ -1,1 +1,5 @@
+func (Rule) Description() string {
+    return "" // documented rule metadata
+}
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected finding for empty string return with trailing comment")
	}
}

func TestSLP100_FiresOnEmptyStringReturnWithTrailingBlockComment(t *testing.T) {
	d := parseDiff(t, `diff --git a/rule.go b/rule.go
--- a/rule.go
+++ b/rule.go
@@ -1,1 +1,5 @@
+func (Rule) Description() string {
+    return "" /* documented rule metadata */
+}
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected finding for empty string return with trailing block comment")
	}
}

func TestSLP100_NoFireOnReturnExpressionAfterBlockComment(t *testing.T) {
	d := parseDiff(t, `diff --git a/rule.go b/rule.go
--- a/rule.go
+++ b/rule.go
@@ -1,1 +1,5 @@
+func (Rule) Value() string {
+    return /* documented rule metadata */ value
+}
`)
	got := SLP100{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for return expression after block comment, got %d: %+v", len(got), got)
	}
}

func TestSLP100_IgnoresDocFiles(t *testing.T) {
	d := parseDiff(t, `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1,1 +1,3 @@
+  func GetItems() { return nil }
`)
	got := SLP100{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for doc files, got %d", len(got))
	}
}

func TestSLP100_Description(t *testing.T) {
	r := SLP100{}
	if r.ID() != "SLP100" {
		t.Errorf("ID = %q", r.ID())
	}
	if r.Description() == "" {
		t.Errorf("Description is empty")
	}
	if r.DefaultSeverity() != SeverityBlock {
		t.Errorf("default severity should be block")
	}
}

func TestSLP100_PythonPassStub(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.py b/handler.py
--- a/handler.py
+++ b/handler.py
@@ -1,1 +1,3 @@
+def get_items():
+    pass
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for Python pass stub")
	}
}

func TestSLP100_PythonRaiseNotImplemented(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.py b/handler.py
--- a/handler.py
+++ b/handler.py
@@ -1,1 +1,3 @@
+def get_items():
+    raise NotImplementedError
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for Python raise NotImplementedError")
	}
}

func TestSLP100_JSArrowStub(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.ts b/handler.ts
--- a/handler.ts
+++ b/handler.ts
@@ -1,1 +1,2 @@
+const getItems = () => null;
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for JS arrow function stub")
	}
}

func TestSLP100_JSThrowUnimplemented(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.ts b/handler.ts
--- a/handler.ts
+++ b/handler.ts
@@ -1,1 +1,3 @@
+function getItems() {
+    throw new Error("not implemented");
+}
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for JS throw not implemented")
	}
}

func TestSLP100_GoPanicUnimplemented(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.go b/handler.go
--- a/handler.go
+++ b/handler.go
@@ -1,1 +1,3 @@
+func GetItems() []Item {
+    panic("not implemented")
+}
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for Go panic not implemented")
	}
}

func TestSLP100_JSConsoleLogStub(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.ts b/handler.ts
--- a/handler.ts
+++ b/handler.ts
@@ -1,1 +1,3 @@
+function getItems() {
+    console.log("todo");
+}
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for console.log stub")
	}
}

func TestSLP100_JSAsyncArrowStub(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.ts b/handler.ts
--- a/handler.ts
+++ b/handler.ts
@@ -1,1 +1,2 @@
+const getItems = async (req) => null;
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for async arrow stub")
	}
}

func TestSLP100_VoidZeroReturn(t *testing.T) {
	d := parseDiff(t, `diff --git a/handler.ts b/handler.ts
--- a/handler.ts
+++ b/handler.ts
@@ -1,1 +1,3 @@
+function getItems() {
+    return void 0;
+}
`)
	got := SLP100{}.Check(d)
	if len(got) == 0 {
		t.Fatal("expected findings for return void 0")
	}
}
