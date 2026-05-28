package rules

import (
	"testing"
)

func TestSLP158_UseEffectFOUC(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want int
	}{
		{
			name: "theme class mutation in useEffect flagged",
			diff: `diff --git a/app.tsx b/app.tsx
--- a/app.tsx
+++ b/app.tsx
@@ -1,4 +1,5 @@
+import { useEffect } from 'react';
 func App() {
-  return null;
+  useEffect(() => {
+    document.documentElement.classList.add('dark');
+  }, []);
 }`,
			want: 1,
		},
		{
			name: "theme setAttribute mutation in useEffect flagged",
			diff: `diff --git a/app.tsx b/app.tsx
--- a/app.tsx
+++ b/app.tsx
@@ -1,4 +1,5 @@
+import { useEffect } from 'react';
 func App() {
-  return null;
+  useEffect(() => {
+    document.documentElement.setAttribute('data-theme', 'dark');
+  }, []);
 }`,
			want: 1,
		},
		{
			name: "theme mutation without useEffect in file ignored",
			diff: `diff --git a/app.tsx b/app.tsx
--- a/app.tsx
+++ b/app.tsx
@@ -1,4 +1,5 @@
 func App() {
-  return null;
+  const x = () => {
+    document.documentElement.classList.add('dark');
+  };
 }`,
			want: 0,
		},
		{
			name: "theme mutation in useEffect context-only flagged",
			diff: `diff --git a/app.tsx b/app.tsx
--- a/app.tsx
+++ b/app.tsx
@@ -2,4 +2,5 @@
   useEffect(() => {
     console.log('hi');
+    document.documentElement.classList.add('dark');
   }, []);
 `,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := parseDiff(t, tt.diff)
			r := SLP158{}
			got := r.Check(d)
			if len(got) != tt.want {
				t.Fatalf("got %d findings, want %d; findings: %v", len(got), tt.want, got)
			}
		})
	}
}
