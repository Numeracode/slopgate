package rules

import (
	"testing"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

func TestSLP162(t *testing.T) {
	longLine := "const reallyLongVariableNameThatExceedsTheRecommendedLengthLimitForCodeQualityAndReadabilityAndWouldBeHardToReviewInADiffBecauseItKeepsGoingAndGoingAndGoingAndGoingAndMore = getValue();"

	tests := []struct {
		name         string
		input        *diff.Diff
		wantFindings int
	}{
		{
			name: "Long line",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: longLine},
								},
							},
						},
					},
				},
			},
			wantFindings: 1,
		},
		{
			name: "Normal code without issues",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "Component.tsx",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: "const value = getValue();"},
								},
							},
						},
					},
				},
			},
			wantFindings: 0,
		},
		{
			name: "Long docs line ignored",
			input: &diff.Diff{
				Files: []diff.File{
					{
						Path: "docs/plan.md",
						Hunks: []diff.Hunk{
							{
								Lines: []diff.Line{
									{Kind: diff.LineAdd, NewLineNo: 1, Content: longLine},
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
			rule := SLP162{}
			findings := rule.Check(tt.input)
			if len(findings) != tt.wantFindings {
				t.Errorf("SLP162 got %d findings, want %d", len(findings), tt.wantFindings)
				for _, f := range findings {
					t.Logf("Finding: %s:%d - %s", f.File, f.Line, f.Message)
				}
			}
		})
	}

	var r SLP162
	if r.DefaultSeverity() != SeverityInfo {
		t.Errorf("SLP162 default severity should be info, got %v", r.DefaultSeverity())
	}
}