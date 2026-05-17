package diff

import (
	"strings"
	"testing"
)

func TestFilterIgnored_NoPatterns_ReturnsAllFiles(t *testing.T) {
	d := &Diff{Files: []File{{Path: "a.go"}, {Path: "b.go"}}}
	out := FilterIgnored(d, nil)
	if len(out.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(out.Files))
	}
}

func TestFilterIgnored_ExactMatch(t *testing.T) {
	d := &Diff{Files: []File{{Path: "a.go"}, {Path: "b.go"}}}
	out := FilterIgnored(d, []string{"a.go"})
	if len(out.Files) != 1 || out.Files[0].Path != "b.go" {
		t.Errorf("expected only b.go, got %+v", out.Files)
	}
}

func TestFilterIgnored_GlobMatch(t *testing.T) {
	d := &Diff{Files: []File{
		{Path: "pkg/rules/slp012_test.go"},
		{Path: "pkg/rules/slp013.go"},
		{Path: "pkg/diff/parser.go"},
	}}
	out := FilterIgnored(d, []string{"pkg/rules/slp*_test.go"})
	paths := pathsOf(out)
	if len(paths) != 2 || paths[0] != "pkg/rules/slp013.go" || paths[1] != "pkg/diff/parser.go" {
		t.Errorf("unexpected paths: %v", paths)
	}
	if len(out.IgnoredFiles) != 1 || out.IgnoredFiles[0].Path != "pkg/rules/slp012_test.go" {
		t.Errorf("unexpected ignored files: %+v", out.IgnoredFiles)
	}
}

func TestFilterIgnored_DoubleStarMatchesAnyDir(t *testing.T) {
	d := &Diff{Files: []File{
		{Path: "pkg/rules/slp012_test.go"},
		{Path: "cmd/slopgate/main.go"},
	}}
	out := FilterIgnored(d, []string{"**/slp*_test.go"})
	paths := pathsOf(out)
	if len(paths) != 1 || paths[0] != "cmd/slopgate/main.go" {
		t.Errorf("unexpected paths: %v", paths)
	}
}

func TestParseIgnoreFile_SkipsCommentsAndBlanks(t *testing.T) {
	content := `
# This is a comment
pkg/rules/slp*_test.go

# Another comment
**/foo_bar.go
`
	got, err := ParseIgnoreFile(strings.NewReader(content))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	want := []string{"pkg/rules/slp*_test.go", "**/foo_bar.go"}
	if len(got) != len(want) {
		t.Fatalf("expected %d patterns, got %d: %v", len(want), len(got), got)
	}
	for i, p := range want {
		if got[i] != p {
			t.Errorf("pattern %d: got %q, want %q", i, got[i], p)
		}
	}
}

func pathsOf(d *Diff) []string {
	out := make([]string, 0, len(d.Files))
	for _, f := range d.Files {
		out = append(out, f.Path)
	}
	return out
}
