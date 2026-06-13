package scorer

import (
	"context"
	"testing"

	"myharness/internal/cases"
)

func TestScorers(t *testing.T) {
	tests := []struct {
		name   string
		spec   cases.ScorerSpec
		output string
		want   bool
	}{
		{"exact match", cases.ScorerSpec{Type: "exact", Value: "Paris"}, "  Paris\n", true},
		{"exact mismatch", cases.ScorerSpec{Type: "exact", Value: "Paris"}, "Paris is the capital", false},
		{"exact ignore case", cases.ScorerSpec{Type: "exact", Value: "Paris", IgnoreCase: true}, "PARIS", true},
		{"contains hit", cases.ScorerSpec{Type: "contains", Value: "Paris"}, "The capital is Paris.", true},
		{"contains miss", cases.ScorerSpec{Type: "contains", Value: "Paris"}, "The capital is Berlin.", false},
		{"contains ignore case", cases.ScorerSpec{Type: "contains", Value: "paris", IgnoreCase: true}, "PARIS", true},
		{"regex hit", cases.ScorerSpec{Type: "regex", Pattern: `\b4\b`}, "2 + 2 = 4", true},
		{"regex miss", cases.ScorerSpec{Type: "regex", Pattern: `\b5\b`}, "2 + 2 = 4", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Build(tt.spec, nil)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if got := s.Score(context.Background(), tt.output).Pass; got != tt.want {
				t.Errorf("Score(%q).Pass = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}

func TestBuildErrors(t *testing.T) {
	if _, err := Build(cases.ScorerSpec{Type: "nope"}, nil); err == nil {
		t.Error("expected error for unknown scorer type")
	}
	if _, err := Build(cases.ScorerSpec{Type: "regex", Pattern: "("}, nil); err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestJudgeScorer(t *testing.T) {
	spec := cases.ScorerSpec{Type: "judge", Rubric: "The answer mentions Paris."}

	// PASS verdict.
	passFn := func(_ context.Context, prompt string) (string, error) { return "PASS", nil }
	s, err := Build(spec, passFn)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if r := s.Score(context.Background(), "Paris is the capital."); !r.Pass {
		t.Errorf("judge PASS: got fail, detail=%q", r.Detail)
	}

	// FAIL verdict with reason.
	failFn := func(_ context.Context, prompt string) (string, error) {
		return "FAIL: does not mention the city", nil
	}
	s, err = Build(spec, failFn)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	r := s.Score(context.Background(), "I don't know.")
	if r.Pass {
		t.Error("judge FAIL: got pass, want fail")
	}
	if r.Detail == "" {
		t.Error("judge FAIL: expected a detail with the reason")
	}

	// Nil judge fails gracefully.
	s, err = Build(spec, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if r := s.Score(context.Background(), "anything"); r.Pass {
		t.Error("nil judge: expected fail")
	}
}
