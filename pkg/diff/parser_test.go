package diff

import (
	"strings"
	"testing"
)

func TestParse_EmptyInput(t *testing.T) {
	d, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(d.Files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(d.Files))
	}
}

func TestParse_SingleFileSingleHunk(t *testing.T) {
	input := `diff --git a/foo.go b/foo.go
index abc..def 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 package foo

-var x = 1
+var x = 2
+var y = 3
`
	d, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(d.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(d.Files))
	}
	f := d.Files[0]
	if f.Path != "foo.go" {
		t.Errorf("expected path foo.go, got %q", f.Path)
	}
	if f.OldPath != "foo.go" {
		t.Errorf("expected old path foo.go, got %q", f.OldPath)
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(f.Hunks))
	}
	h := f.Hunks[0]
	if h.NewStart != 1 || h.NewLines != 4 {
		t.Errorf("new hunk header wrong: start=%d lines=%d", h.NewStart, h.NewLines)
	}

	// Expected lines: context "package foo", context "", delete "var x = 1", add "var x = 2", add "var y = 3"
	if len(h.Lines) != 5 {
		t.Fatalf("expected 5 diff lines, got %d", len(h.Lines))
	}

	adds := 0
	dels := 0
	for _, ln := range h.Lines {
		switch ln.Kind {
		case LineAdd:
			adds++
		case LineDelete:
			dels++
		}
	}
	if adds != 2 || dels != 1 {
		t.Errorf("expected 2 add 1 del, got %d add %d del", adds, dels)
	}

	// Verify new line numbers are correct on added lines
	var addLineNos []int
	for _, ln := range h.Lines {
		if ln.Kind == LineAdd {
			addLineNos = append(addLineNos, ln.NewLineNo)
		}
	}
	if len(addLineNos) != 2 || addLineNos[0] != 3 || addLineNos[1] != 4 {
		t.Errorf("expected added line numbers [3 4], got %v", addLineNos)
	}
}

func TestParse_NewFile(t *testing.T) {
	input := `diff --git a/new.go b/new.go
new file mode 100644
index 0000000..1111111
--- /dev/null
+++ b/new.go
@@ -0,0 +1,2 @@
+package new
+
`
	d, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(d.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(d.Files))
	}
	f := d.Files[0]
	if !f.IsNew {
		t.Errorf("expected IsNew=true")
	}
	if f.Path != "new.go" {
		t.Errorf("expected path new.go, got %q", f.Path)
	}
}

func TestParse_DeletedFile(t *testing.T) {
	input := `diff --git a/gone.go b/gone.go
deleted file mode 100644
index 1111111..0000000
--- a/gone.go
+++ /dev/null
@@ -1,2 +0,0 @@
-package gone
-
`
	d, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(d.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(d.Files))
	}
	f := d.Files[0]
	if !f.IsDelete {
		t.Errorf("expected IsDelete=true")
	}
	if f.OldPath != "gone.go" {
		t.Errorf("expected old path gone.go, got %q", f.OldPath)
	}
}

func TestDiffAllFilesIncludesIgnoredFiles(t *testing.T) {
	d := &Diff{
		Files:        []File{{Path: "pkg/rules/slp130.go"}},
		ignoredFiles: []File{{Path: "pkg/rules/slp130_test.go"}},
	}
	got := d.AllFiles()
	if len(got) != 2 {
		t.Fatalf("expected 2 files, got %d", len(got))
	}
	if got[0].Path != "pkg/rules/slp130.go" || got[1].Path != "pkg/rules/slp130_test.go" {
		t.Fatalf("unexpected files from AllFiles: %+v", got)
	}
}

func TestParse_MultipleFilesMultipleHunks(t *testing.T) {
	input := `diff --git a/a.go b/a.go
index aaa..bbb 100644
--- a/a.go
+++ b/a.go
@@ -10,3 +10,4 @@
 context1
-old
+new1
+new2
 context2
@@ -50,2 +51,3 @@
 context3
+added
 context4
diff --git a/b.py b/b.py
index ccc..ddd 100644
--- a/b.py
+++ b/b.py
@@ -1,2 +1,3 @@
 first
+second
 third
`
	d, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(d.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(d.Files))
	}
	if len(d.Files[0].Hunks) != 2 {
		t.Errorf("expected 2 hunks in a.go, got %d", len(d.Files[0].Hunks))
	}
	if d.Files[0].Hunks[1].NewStart != 51 {
		t.Errorf("expected second hunk of a.go at new line 51, got %d", d.Files[0].Hunks[1].NewStart)
	}
	if d.Files[1].Path != "b.py" {
		t.Errorf("expected second file b.py, got %q", d.Files[1].Path)
	}
}

func TestParse_AddedLineNumbersTrackCorrectly(t *testing.T) {
	input := `diff --git a/x.go b/x.go
index 1..2 100644
--- a/x.go
+++ b/x.go
@@ -5,4 +5,6 @@
 line5
 line6
+new7
+new8
 line7_old
-line8_old
+new9
`
	d, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	h := d.Files[0].Hunks[0]

	var got [][2]any
	for _, ln := range h.Lines {
		got = append(got, [2]any{ln.Kind, ln.NewLineNo})
	}
	// new numbering in hunk: 5 (context), 6 (context), 7 (add), 8 (add), 9 (context), -- (del), 10 (add)
	expected := [][2]any{
		{LineContext, 5},
		{LineContext, 6},
		{LineAdd, 7},
		{LineAdd, 8},
		{LineContext, 9},
		{LineDelete, 0},
		{LineAdd, 10},
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %d lines, got %d", len(expected), len(got))
	}
	for i, e := range expected {
		if got[i][0] != e[0] || got[i][1] != e[1] {
			t.Errorf("line %d: expected %v, got %v", i, e, got[i])
		}
	}
}

func TestAddedLines_Helper(t *testing.T) {
	input := `diff --git a/x.go b/x.go
--- a/x.go
+++ b/x.go
@@ -1,3 +1,4 @@
 keep
-old
+new1
+new2
 keep2
`
	d, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	added := d.Files[0].AddedLines()
	if len(added) != 2 {
		t.Fatalf("expected 2 added lines, got %d", len(added))
	}
	if added[0].Content != "new1" || added[1].Content != "new2" {
		t.Errorf("unexpected added content: %+v", added)
	}
	if added[0].NewLineNo != 2 || added[1].NewLineNo != 3 {
		t.Errorf("unexpected added line nos: %d %d", added[0].NewLineNo, added[1].NewLineNo)
	}
}
