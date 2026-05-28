package rules

import (
	"testing"
)

func TestSLP157_ParseIntFloatTruncation(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want int
	}{
		{
			name: "parseInt on query parameter flagged",
			diff: `diff --git a/handler.js b/handler.js
--- a/handler.js
+++ b/handler.js
@@ -1,2 +1,3 @@
+const limit = parseInt(req.query.limit);
 `,
			want: 1,
		},
		{
			name: "parseInt on body parameter flagged",
			diff: `diff --git a/handler.js b/handler.js
--- a/handler.js
+++ b/handler.js
@@ -1,2 +1,3 @@
+const id = parseInt(req.body.id);
 `,
			want: 1,
		},
		{
			name: "parseInt with Zod validation inline skipped",
			diff: `diff --git a/handler.js b/handler.js
--- a/handler.js
+++ b/handler.js
@@ -1,2 +1,3 @@
+const limit = parseInt(req.query.limit); // schema.validate
 `,
			want: 0,
		},
		{
			name: "parseInt with Zod validation on separate line in same hunk skipped",
			diff: `diff --git canvas.js canvas.js
--- a/canvas.js
+++ b/canvas.js
@@ -1,3 +1,5 @@
+if (schema.validate(req.query.limit)) {
+  const limit = parseInt(req.query.limit);
+}
+`,
			want: 0,
		},
		{
			name: "parseInt on generic non-payload variable ignored",
			diff: `diff --git a/handler.js b/handler.js
--- a/handler.js
+++ b/handler.js
@@ -1,2 +1,3 @@
+const limit = parseInt(x);
 `,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := parseDiff(t, tt.diff)
			r := SLP157{}
			got := r.Check(d)
			if len(got) != tt.want {
				t.Fatalf("got %d findings, want %d; findings: %v", len(got), tt.want, got)
			}
		})
	}
}
