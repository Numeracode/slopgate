package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

func TestSLP033(t *testing.T) {
	tests := []struct {
		name         string
		input        *diff.Diff
		wantFindings int
	}{
		{
			name: "useState used without import",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "const [count, setCount] = useState(0);"},
								},
							},
						},
					},
				},
			},
			wantFindings: 1,
		},
		{
			name: "useState used with import",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "import { useState } from 'react';"},
									{Kind: diff.LineAdd, NewLineNo: 2, Content: "const [count, setCount] = useState(0);"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "Type used in type annotation without import",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "function MyComponent(props: ComponentProps) {"},
								},
							},
						},
					},
				},
			},
			wantFindings: 1,
		},
		{
			name: "Type used with import",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "import { ComponentProps } from 'react';"},
									{Kind: diff.LineAdd, NewLineNo: 2, Content: "function MyComponent(props: ComponentProps) {"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "Non-JS/TS file",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "main.go",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "useState := 0"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "Namespace import satisfies React availability",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "import * as React from 'react';"},
									{Kind: diff.LineAdd, NewLineNo: 2, Content: "const [count, setCount] = React.useState(0);"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "Import in different hunk",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "import { useState } from 'react';"},
								},
							},
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 5, Content: "const [count, setCount] = useState(0);"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := SLP033{}
			findings := rule.Check(tt.input)
			if len(findings) != tt.wantFindings {
				t.Errorf("SLP033 got %d findings, want %d", len(findings), tt.wantFindings)
				for _, f := range findings {
					t.Logf("Finding: %s:%d - %s", f.File, f.Line, f.Message)
				}
			}
		})
	}
}

func TestSLP033_ContextTypeOnlyImportAvoidsFalsePositive(t *testing.T) {
	d := parseDiff(t, `diff --git a/app/src/components/docs/MDXProvider.tsx b/app/src/components/docs/MDXProvider.tsx
--- a/app/src/components/docs/MDXProvider.tsx
+++ b/app/src/components/docs/MDXProvider.tsx
@@ -1,3 +1,4 @@
 import type { ReactNode } from 'react'
 
+type ProviderProps = { children: ReactNode }
 export function MDXProvider({ children }: ProviderProps) {
`)

	got := SLP033{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for context type import, got %d: %+v", len(got), got)
	}
}

func TestSLP033_MultilineTypeImportAndAliasAvoidFalsePositive(t *testing.T) {
	d := parseDiff(t, `diff --git a/app/src/components/docs/MDXProvider.tsx b/app/src/components/docs/MDXProvider.tsx
--- a/app/src/components/docs/MDXProvider.tsx
+++ b/app/src/components/docs/MDXProvider.tsx
@@ -1,5 +1,6 @@
 import {
   type ReactNode,
   type ComponentType as MDXComponent,
 } from 'react'
 
+type ProviderProps = { children: ReactNode; components?: MDXComponent<Record<string, unknown>> }
`)

	got := SLP033{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings for multiline type import, got %d: %+v", len(got), got)
	}
}

func TestSLP033_FallsBackToSnapshotFileImportsOutsideHunk(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "app/src/components/docs")
	if err := os.MkdirAll(filePath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := `import type { ReactNode } from "react";
import { Link } from "react-router-dom";

function Callout({ children }: { children?: ReactNode }) {
  return <aside>{children}</aside>;
}
`
	target := filepath.Join(filePath, "MDXProvider.tsx")
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	d := parseDiffWithRoot(t, root, `diff --git a/app/src/components/docs/MDXProvider.tsx b/app/src/components/docs/MDXProvider.tsx
--- a/app/src/components/docs/MDXProvider.tsx
+++ b/app/src/components/docs/MDXProvider.tsx
@@ -4,2 +4,5 @@
-function Callout({ children }: { children?: ReactNode }) {
-  return <aside>{children}</aside>;
+function Callout({ children }: { children?: ReactNode }) {
+  const className = "callout";
+  return <aside className={className}>{children}</aside>;
+}
`)

	got := SLP033{}.Check(d)
	if len(got) != 0 {
		t.Fatalf("expected 0 findings with snapshot file imports, got %d: %+v", len(got), got)
	}
}
