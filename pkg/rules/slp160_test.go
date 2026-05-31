package rules

import (
	"testing"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

func TestSLP160(t *testing.T) {
	tests := []struct {
		name         string
		input        *diff.Diff
		wantFindings int
	}{
		{
			name: "TODO without ticket reference",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "// TODO: fix this later"},
								},
							},
						},
					},
				},
			},
			wantFindings: 1,
		},
		{
			name: "TODO with ticket reference",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "// TODO: CR-123 fix this later"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "FIXME without ticket reference",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "app.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "// FIXME: this is broken"},
								},
							},
						},
					},
				},
			},
			wantFindings: 1,
		},
		{
			name: "TODO in test file skipped",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.test.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "// TODO: update this test"},
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
			rule := SLP160{}
			findings := rule.Check(tt.input)
			if len(findings) != tt.wantFindings {
				t.Errorf("SLP160 got %d findings, want %d", len(findings), tt.wantFindings)
				for _, f := range findings {
					t.Logf("Finding: %s:%d - %s", f.File, f.Line, f.Message)
				}
			}
		})
	}

	var r SLP160
	if r.DefaultSeverity() != SeverityInfo {
		t.Errorf("SLP160 default severity should be info, got %v", r.DefaultSeverity())
	}
}