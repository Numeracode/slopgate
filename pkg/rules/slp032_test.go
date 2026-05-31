package rules

import (
	"testing"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

func TestSLP032(t *testing.T) {
	tests := []struct {
		name         string
		input        *diff.Diff
		wantFindings int
	}{
		{
			name: "TSX component without React import (no hooks - not flagged in React 17+)",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "export function MyComponent() {"},
									{Kind: diff.LineAdd, NewLineNo: 2, Content: "  return <div>Hello</div>;"},
									{Kind: diff.LineAdd, NewLineNo: 3, Content: "}"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "TSX with React import",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "import React from 'react';"},
									{Kind: diff.LineAdd, NewLineNo: 2, Content: "export function MyComponent() {"},
									{Kind: diff.LineAdd, NewLineNo: 3, Content: "  return <div>Hello</div>;"},
									{Kind: diff.LineAdd, NewLineNo: 4, Content: "}"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "Button with visible text is accessible",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "import React from 'react';"},
									{Kind: diff.LineAdd, NewLineNo: 2, Content: "export function MyComponent() {"},
									{Kind: diff.LineAdd, NewLineNo: 3, Content: "  return <button>Click me</button>;"},
									{Kind: diff.LineAdd, NewLineNo: 4, Content: "}"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "Button without aria, title, or visible text",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "import React from 'react';"},
									{Kind: diff.LineAdd, NewLineNo: 2, Content: "export function MyComponent() {"},
									{Kind: diff.LineAdd, NewLineNo: 3, Content: "  return <button />;"},
									{Kind: diff.LineAdd, NewLineNo: 4, Content: "}"},
								},
							},
						},
					},
				},
			},
			wantFindings: 1,
		},
		{
			name: "Button with aria-label is accessible",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "import React from 'react';"},
									{Kind: diff.LineAdd, NewLineNo: 2, Content: "export function MyComponent() {"},
									{Kind: diff.LineAdd, NewLineNo: 3, Content: "  return <button aria-label='Close' />;"},
									{Kind: diff.LineAdd, NewLineNo: 4, Content: "}"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "React import in different hunk",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "import React from 'react';"},
								},
							},
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 5, Content: "export function MyComponent() {"},
									{Kind: diff.LineAdd, NewLineNo: 6, Content: "  return <div>Hello</div>;"},
									{Kind: diff.LineAdd, NewLineNo: 7, Content: "}"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "react-router-dom should not count as React import",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "import { Link } from 'react-router-dom';"},
									{Kind: diff.LineAdd, NewLineNo: 2, Content: "export function MyComponent() {"},
									{Kind: diff.LineAdd, NewLineNo: 3, Content: "  return <div>Hello</div>;"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "useState without React import",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "import { Link } from 'react-router-dom';"},
									{Kind: diff.LineAdd, NewLineNo: 2, Content: "export function MyComponent() {"},
									{Kind: diff.LineAdd, NewLineNo: 3, Content: "  const [val, setVal] = useState(0);"},
								},
							},
						},
					},
				},
			},
			wantFindings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := SLP032{}
			findings := rule.Check(tt.input)
			if len(findings) != tt.wantFindings {
				t.Errorf("SLP032 got %d findings, want %d", len(findings), tt.wantFindings)
				for _, f := range findings {
					t.Logf("Finding: %s:%d - %s", f.File, f.Line, f.Message)
				}
			}
		})
	}
}
