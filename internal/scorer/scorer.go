// Package scorer judges a model's text response. Each scorer implements one
// way of deciding pass/fail. New scorer types plug in through the registry
// without touching the runner.
package scorer

import (
	"context"
	"fmt"

	"myharness/internal/cases"
)

// Result is the outcome of one scorer judging one response.
type Result struct {
	Pass   bool    `json:"pass"`
	Score  float64 `json:"score"`  // 0..1; 1.0 == pass for the boolean scorers
	Detail string  `json:"detail"` // human-readable explanation
}

// Scorer judges a response. Implementations are built from a cases.ScorerSpec
// via Build, so they carry their own configuration.
type Scorer interface {
	// Score judges the model output and reports the outcome. The context lets
	// scorers that call out to a model (e.g. judge) honour cancellation;
	// deterministic scorers ignore it.
	Score(ctx context.Context, output string) Result
	// Describe returns a short label used in reports, e.g. `contains "Paris"`.
	Describe() string
}

// JudgeFunc asks a model to evaluate a prompt and returns its text reply. It is
// supplied by the runner (built from the configured provider) so the judge
// scorer stays decoupled from any specific backend. A nil JudgeFunc means no
// model is available and judge scorers fail gracefully.
type JudgeFunc func(ctx context.Context, prompt string) (string, error)

// Build constructs a Scorer from its JSON spec. This is the single place that
// maps a spec "type" to a concrete scorer; adding a new scorer means adding a
// case here. The judge function is threaded through for scorers that need a
// model; deterministic scorers ignore it.
func Build(spec cases.ScorerSpec, judge JudgeFunc) (Scorer, error) {
	switch spec.Type {
	case "exact":
		return &exact{want: spec.Value, ignoreCase: spec.IgnoreCase}, nil
	case "contains":
		return &contains{want: spec.Value, ignoreCase: spec.IgnoreCase}, nil
	case "regex":
		return newRegex(spec.Pattern)
	case "judge":
		return &judgeScorer{rubric: spec.Rubric, fn: judge}, nil
	default:
		return nil, fmt.Errorf("unknown scorer type %q", spec.Type)
	}
}

// BuildAll builds every scorer for a case, failing fast on the first bad spec.
func BuildAll(specs []cases.ScorerSpec, judge JudgeFunc) ([]Scorer, error) {
	out := make([]Scorer, 0, len(specs))
	for i, s := range specs {
		sc, err := Build(s, judge)
		if err != nil {
			return nil, fmt.Errorf("scorer #%d: %w", i+1, err)
		}
		out = append(out, sc)
	}
	return out, nil
}
