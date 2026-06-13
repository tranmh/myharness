package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"myharness/internal/cases"
	"myharness/internal/provider"
)

// scriptProvider returns a sequence of responses on each call, so we can model
// a phase that fails then passes. It records the prompts it was asked.
type scriptProvider struct {
	replies []string
	prompts []string
	i       int
}

func (s *scriptProvider) Run(_ context.Context, r provider.Request) (provider.Response, error) {
	s.prompts = append(s.prompts, r.Prompt)
	text := ""
	if s.i < len(s.replies) {
		text = s.replies[s.i]
	} else if len(s.replies) > 0 {
		text = s.replies[len(s.replies)-1]
	}
	s.i++
	return provider.Response{Text: text, NumTurns: 1, CostUSD: 0.01}, nil
}

// scriptPrompter returns a fixed sequence of decisions on give-up.
type scriptPrompter struct {
	decisions []Decision
	i         int
}

func (p *scriptPrompter) OnGiveUp(_, _ string, _ int, _ []string) Decision {
	d := DecisionSkip
	if p.i < len(p.decisions) {
		d = p.decisions[p.i]
	}
	p.i++
	return d
}

func TestRetryThenPassWithFeedback(t *testing.T) {
	// Attempt 1 misses "DONE"; attempt 2 includes it.
	p := &scriptProvider{replies: []string{"nope", "all DONE here"}}
	cs := []cases.Case{{
		Name: "retry",
		Phases: []cases.Phase{{
			Name:    "p1",
			Prompt:  "do the thing",
			Scorers: []cases.ScorerSpec{{Type: "contains", Value: "DONE"}},
		}},
	}}

	got := Run(context.Background(), p, cs, Options{Parallel: 1, MaxTries: 5})
	if len(got) != 1 || !got[0].Pass {
		t.Fatalf("expected pass, got %+v", got)
	}
	if len(p.prompts) != 2 {
		t.Fatalf("expected 2 provider calls, got %d", len(p.prompts))
	}
	// The second prompt must carry the failure feedback.
	if !strings.Contains(p.prompts[1], "previous attempt failed") {
		t.Errorf("attempt 2 prompt missing feedback: %q", p.prompts[1])
	}
	if !strings.Contains(p.prompts[1], "do the thing") {
		t.Errorf("attempt 2 prompt missing original prompt: %q", p.prompts[1])
	}
	pr := got[0].Phases[0]
	if pr.Tries != 2 {
		t.Errorf("expected 2 tries, got %d", pr.Tries)
	}
}

func TestGiveUpSkip(t *testing.T) {
	p := &scriptProvider{replies: []string{"never matches"}}
	cs := []cases.Case{{
		Name: "skip",
		Phases: []cases.Phase{{
			Name:    "p1",
			Prompt:  "x",
			Scorers: []cases.ScorerSpec{{Type: "contains", Value: "IMPOSSIBLE"}},
		}},
	}}

	got := Run(context.Background(), p, cs, Options{
		Parallel: 1, MaxTries: 2, Prompter: &scriptPrompter{decisions: []Decision{DecisionSkip}},
	})
	if got[0].Pass {
		t.Error("expected case to fail after skip")
	}
	pr := got[0].Phases[0]
	if pr.Tries != 2 {
		t.Errorf("expected 2 tries, got %d", pr.Tries)
	}
	if pr.Decision != "skip" {
		t.Errorf("expected skip decision, got %q", pr.Decision)
	}
}

func TestBudgetExceededFailsCase(t *testing.T) {
	// Each call costs 0.01; cap at 0.005 so the very first call blows it.
	p := &scriptProvider{replies: []string{"DONE"}}
	cs := []cases.Case{{
		Name:       "budget",
		MaxCostUSD: 0.005,
		Phases: []cases.Phase{{
			Name:    "p1",
			Prompt:  "x",
			Scorers: []cases.ScorerSpec{{Type: "contains", Value: "DONE"}},
		}},
	}}

	got := Run(context.Background(), p, cs, Options{Parallel: 1, MaxTries: 1})
	if got[0].Pass {
		t.Error("expected case to fail on budget")
	}
	if !strings.Contains(got[0].Err, "budget exceeded") {
		t.Errorf("expected budget error, got %q", got[0].Err)
	}
}

func TestLegacySinglePromptNormalizes(t *testing.T) {
	dir := t.TempDir()
	casePath := filepath.Join(dir, "legacy.json")
	if err := os.WriteFile(casePath, []byte(
		`{"name":"legacy","prompt":"make a page","save_to":"out.html","scorers":[{"type":"contains","value":"<canvas"}]}`,
	), 0o644); err != nil {
		t.Fatal(err)
	}

	cs, err := cases.Load(casePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cs) != 1 || len(cs[0].Phases) != 1 {
		t.Fatalf("expected 1 case with 1 phase, got %+v", cs)
	}
	if cs[0].Phases[0].Name != "phase-1" {
		t.Errorf("expected default name phase-1, got %q", cs[0].Phases[0].Name)
	}

	p := &provider.Mock{Default: "Here you go:\n```html\n<canvas></canvas>\n```"}
	got := Run(context.Background(), p, cs, Options{Parallel: 1})
	if !got[0].Pass {
		t.Fatalf("legacy case should pass, got %+v", got[0])
	}
	pr := got[0].Phases[0]
	if pr.SavedTo == "" {
		t.Fatal("expected file to be saved")
	}
	data, err := os.ReadFile(pr.SavedTo)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "<canvas></canvas>\n" {
		t.Errorf("saved file = %q, want extracted canvas", string(data))
	}
}

func TestAbortCaseStopsLaterPhases(t *testing.T) {
	p := &scriptProvider{replies: []string{"never"}}
	cs := []cases.Case{{
		Name: "abort",
		Phases: []cases.Phase{
			{Name: "p1", Prompt: "x", Scorers: []cases.ScorerSpec{{Type: "contains", Value: "NOPE"}}},
			{Name: "p2", Prompt: "y", Scorers: []cases.ScorerSpec{{Type: "contains", Value: "never"}}},
		},
	}}

	got := Run(context.Background(), p, cs, Options{
		Parallel: 1, MaxTries: 1, Prompter: &scriptPrompter{decisions: []Decision{DecisionAbortCase}},
	})
	if got[0].Pass {
		t.Error("expected fail")
	}
	if len(got[0].Phases) != 1 {
		t.Errorf("expected only phase 1 to run, got %d phases", len(got[0].Phases))
	}
}

func TestExtractCode(t *testing.T) {
	cs := []struct{ in, want string }{
		{"```html\n<h1>hi</h1>\n```", "<h1>hi</h1>\n"},
		{"no fences here", "no fences here\n"},
		{"```\nplain\n```", "plain\n"},
	}
	for _, tc := range cs {
		if got := extractCode(tc.in); got != tc.want {
			t.Errorf("extractCode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
