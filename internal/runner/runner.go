// Package runner ties the pieces together: for each case it runs its setup,
// drives each phase (a fresh provider call retried with feedback until its
// scorers pass or we give up), optionally saves phase output to a file, runs
// teardown, and collects the outcomes. Cases run concurrently through a worker
// pool.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"myharness/internal/cases"
	"myharness/internal/provider"
	"myharness/internal/scorer"
)

// JudgeFunc is the model-callout used by judge scorers, re-exported here so
// main.go can build one without importing the scorer package directly.
type JudgeFunc = scorer.JudgeFunc

// Decision is what the operator (or CI) chooses when a phase runs out of tries.
type Decision int

const (
	// DecisionSkip marks the phase failed but lets the rest of the case run.
	DecisionSkip Decision = iota
	// DecisionRetry grants the phase a fresh budget of tries.
	DecisionRetry
	// DecisionAbortCase stops the current case but lets other cases run.
	DecisionAbortCase
	// DecisionQuitRun cancels the whole run.
	DecisionQuitRun
)

func (d Decision) String() string {
	switch d {
	case DecisionRetry:
		return "retry"
	case DecisionSkip:
		return "skip"
	case DecisionAbortCase:
		return "abort"
	case DecisionQuitRun:
		return "quit"
	default:
		return "unknown"
	}
}

// Prompter decides what to do when a phase exhausts its retries.
type Prompter interface {
	// OnGiveUp is called when a phase has failed `tries` attempts. failed lists
	// the descriptions of the scorers that did not pass on the last attempt.
	OnGiveUp(caseName, phaseName string, tries int, failed []string) Decision
}

// ScoreResult pairs one scorer's description with its outcome.
type ScoreResult struct {
	Describe string        `json:"scorer"`
	Result   scorer.Result `json:"result"`
}

// AttemptResult is one provider call plus its scoring.
type AttemptResult struct {
	Output   string        `json:"output"`
	Scores   []ScoreResult `json:"scores"`
	Pass     bool          `json:"pass"`
	CostUSD  float64       `json:"cost_usd"`
	NumTurns int           `json:"num_turns"`
	Duration time.Duration `json:"duration"`
	Err      string        `json:"error,omitempty"`
}

// PhaseResult collects every attempt for one phase.
type PhaseResult struct {
	Name     string          `json:"name"`
	Pass     bool            `json:"pass"`
	Tries    int             `json:"tries"`
	Attempts []AttemptResult `json:"attempts"`
	SavedTo  string          `json:"saved_to,omitempty"`
	Decision string          `json:"decision,omitempty"` // give-up decision, if any
}

// CaseResult is everything we learned from running one case. Cost/turns/
// duration are summed across all phases and attempts.
type CaseResult struct {
	Name     string        `json:"name"`
	Tags     []string      `json:"tags,omitempty"`
	Pass     bool          `json:"pass"`
	Phases   []PhaseResult `json:"phases"`
	CostUSD  float64       `json:"cost_usd"`
	NumTurns int           `json:"num_turns"`
	Duration time.Duration `json:"duration"`
	Err      string        `json:"error,omitempty"`
}

// Options configures a run.
type Options struct {
	Parallel      int           // max concurrent cases (default 4)
	ModelOverride string        // if set, overrides each case/phase model
	Timeout       time.Duration // per-provider-call timeout (default 120s)
	MaxTries      int           // retries per phase (default 5)
	Prompter      Prompter      // give-up handler; defaults to non-interactive (skip)
	Judge         JudgeFunc     // model callout for judge scorers; may be nil
}

// Run executes every case and returns the results in the same order as input.
func Run(ctx context.Context, p provider.Provider, cs []cases.Case, opts Options) []CaseResult {
	if opts.Parallel <= 0 {
		opts.Parallel = 4
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 120 * time.Second
	}
	if opts.MaxTries <= 0 {
		opts.MaxTries = 5
	}
	if opts.Prompter == nil {
		opts.Prompter = NoninteractivePrompter{}
	}

	// A cancelable context so a QuitRun decision stops other in-flight cases.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]CaseResult, len(cs))
	sem := make(chan struct{}, opts.Parallel)
	var wg sync.WaitGroup

	for i, c := range cs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, c cases.Case) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = runOne(runCtx, p, c, opts, cancel)
		}(i, c)
	}
	wg.Wait()
	return results
}

// runOne runs a case's setup, every phase, and teardown.
func runOne(ctx context.Context, p provider.Provider, c cases.Case, opts Options, quit context.CancelFunc) CaseResult {
	res := CaseResult{Name: c.Name, Tags: c.Tags, Pass: true}

	// Resolve the workdir once: a relative workdir resolves against the case
	// file's directory, so artifacts land beside the case (e.g. cases/out/...).
	workDir := effectiveWorkDir(c)

	// Create the workdir up front so setup/provider/save_to all see it.
	if workDir != "" {
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			res.Pass = false
			res.Err = fmt.Sprintf("workdir %s: %v", workDir, err)
			return res
		}
	}

	// Teardown always runs, even if a phase fails or aborts.
	defer func() {
		if err := runShell(ctx, c.Teardown, workDir); err != nil && res.Err == "" {
			res.Err = fmt.Sprintf("teardown: %v", err)
		}
	}()

	if err := runShell(ctx, c.Setup, workDir); err != nil {
		res.Pass = false
		res.Err = fmt.Sprintf("setup: %v", err)
		return res
	}

	// budget tracks cumulative cost/turns across all phases and attempts.
	var totalCost float64
	var totalTurns int
	prevOutput := ""
	outByName := map[string]string{}

	for pi := range c.Phases {
		ph := c.Phases[pi]
		pr := runPhase(ctx, p, c, ph, workDir, opts, prevOutput, outByName, &totalCost, &totalTurns, quit)
		res.Phases = append(res.Phases, pr)
		res.CostUSD = totalCost
		res.NumTurns = totalTurns
		for _, a := range pr.Attempts {
			res.Duration += a.Duration
		}

		if !pr.Pass {
			res.Pass = false
		}
		prevOutput = lastOutput(pr)
		outByName[ph.Name] = prevOutput

		// Budget enforcement: if a cap is exceeded, fail the whole case and stop.
		if c.MaxCostUSD > 0 && totalCost > c.MaxCostUSD {
			res.Pass = false
			res.Err = fmt.Sprintf("budget exceeded: cost $%.4f > $%.4f", totalCost, c.MaxCostUSD)
			return res
		}
		if c.MaxTurnsTotal > 0 && totalTurns > c.MaxTurnsTotal {
			res.Pass = false
			res.Err = fmt.Sprintf("budget exceeded: turns %d > %d", totalTurns, c.MaxTurnsTotal)
			return res
		}

		// Stop early on abort/quit decisions or a cancelled run context.
		if pr.Decision == DecisionAbortCase.String() {
			return res
		}
		if pr.Decision == DecisionQuitRun.String() || ctx.Err() != nil {
			if res.Err == "" {
				res.Err = "run cancelled"
			}
			return res
		}
	}

	return res
}

// runPhase drives one phase: call the provider, score, and retry with feedback
// until the scorers pass or we exhaust the try budget (and the operator gives
// up).
func runPhase(ctx context.Context, p provider.Provider, c cases.Case, ph cases.Phase, workDir string, opts Options,
	prevOutput string, outByName map[string]string, totalCost *float64, totalTurns *int, quit context.CancelFunc) PhaseResult {

	pr := PhaseResult{Name: ph.Name}

	tries := ph.MaxTries
	if tries <= 0 {
		tries = opts.MaxTries
	}

	model := ph.Model
	if opts.ModelOverride != "" {
		model = opts.ModelOverride
	}

	scorers, err := scorer.BuildAll(ph.Scorers, opts.Judge)
	if err != nil {
		pr.Attempts = append(pr.Attempts, AttemptResult{Err: err.Error()})
		pr.Tries = 1
		return pr
	}

	basePrompt, err := renderPrompt(ph.Prompt, prevOutput, outByName)
	if err != nil {
		pr.Attempts = append(pr.Attempts, AttemptResult{Err: err.Error()})
		pr.Tries = 1
		return pr
	}

	for {
		var failedDesc []string

		for attempt := 1; attempt <= tries; attempt++ {
			if ctx.Err() != nil {
				pr.Decision = DecisionQuitRun.String()
				return pr
			}

			prompt := basePrompt
			if attempt > 1 && len(failedDesc) > 0 {
				prompt = basePrompt + "\n\n" + feedback(failedDesc)
			}

			ar, failed := runAttempt(ctx, p, c, ph, workDir, model, prompt, scorers, opts.Timeout)
			*totalCost += ar.CostUSD
			*totalTurns += ar.NumTurns
			pr.Attempts = append(pr.Attempts, ar)
			pr.Tries = len(pr.Attempts)
			failedDesc = failed

			if ar.Pass {
				pr.Pass = true
				if ph.SaveTo != "" {
					path := resolveSaveTo(c, workDir, ph.SaveTo)
					if werr := os.WriteFile(path, []byte(extractCode(ar.Output)), 0o644); werr != nil {
						// Record the save error on the attempt but keep the pass.
						pr.Attempts[len(pr.Attempts)-1].Err = fmt.Sprintf("save_to %s: %v", path, werr)
					} else {
						pr.SavedTo = path
					}
				}
				return pr
			}
		}

		// Out of tries: ask the operator what to do.
		decision := opts.Prompter.OnGiveUp(c.Name, ph.Name, tries, failedDesc)
		pr.Decision = decision.String()
		switch decision {
		case DecisionRetry:
			continue // grant a fresh budget of `tries` attempts
		case DecisionQuitRun:
			if quit != nil {
				quit()
			}
			return pr
		default: // Skip or AbortCase: phase fails, caller handles the rest
			return pr
		}
	}
}

// runAttempt performs one provider call and scores its output. It returns the
// attempt result and the descriptions of any scorers that failed.
func runAttempt(ctx context.Context, p provider.Provider, c cases.Case, ph cases.Phase, workDir, model, prompt string,
	scorers []scorer.Scorer, timeout time.Duration) (AttemptResult, []string) {

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := p.Run(callCtx, provider.Request{
		Prompt:       prompt,
		Model:        model,
		System:       ph.System,
		AllowedTools: ph.AllowedTools,
		MaxTurns:     ph.MaxTurns,
		WorkDir:      workDir,
	})
	if err != nil {
		return AttemptResult{Err: err.Error()}, nil
	}

	ar := AttemptResult{
		Output:   resp.Text,
		CostUSD:  resp.CostUSD,
		NumTurns: resp.NumTurns,
		Duration: resp.Duration,
	}
	if resp.IsError {
		ar.Err = "provider reported is_error=true"
		return ar, nil
	}

	allPass := true
	var failed []string
	for _, s := range scorers {
		r := s.Score(callCtx, resp.Text)
		if !r.Pass {
			allPass = false
			failed = append(failed, s.Describe()+": "+r.Detail)
		}
		ar.Scores = append(ar.Scores, ScoreResult{Describe: s.Describe(), Result: r})
	}
	ar.Pass = allPass
	return ar, failed
}

// feedback builds the note appended to a retried prompt so the model can self-
// correct.
func feedback(failed []string) string {
	var b strings.Builder
	b.WriteString("Your previous attempt failed these checks:\n")
	for _, f := range failed {
		b.WriteString("- ")
		b.WriteString(f)
		b.WriteString("\n")
	}
	b.WriteString("Please fix these issues and output the full result again.")
	return b.String()
}

// renderPrompt runs a phase prompt through text/template with .Prev (previous
// phase output) and .Out (map of phase name -> output). A prompt with no
// template markers is returned unchanged.
func renderPrompt(prompt, prev string, outByName map[string]string) (string, error) {
	if !strings.Contains(prompt, "{{") {
		return prompt, nil
	}
	tmpl, err := template.New("prompt").Parse(prompt)
	if err != nil {
		return "", fmt.Errorf("prompt template: %w", err)
	}
	var buf bytes.Buffer
	data := struct {
		Prev string
		Out  map[string]string
	}{Prev: prev, Out: outByName}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("prompt template: %w", err)
	}
	return buf.String(), nil
}

// lastOutput returns the output of the final attempt in a phase, if any.
func lastOutput(pr PhaseResult) string {
	if len(pr.Attempts) == 0 {
		return ""
	}
	return pr.Attempts[len(pr.Attempts)-1].Output
}

// runShell runs a list of shell commands with bash -lc, in dir if set. It stops
// at the first failure.
func runShell(ctx context.Context, cmds []string, dir string) error {
	for _, c := range cmds {
		if strings.TrimSpace(c) == "" {
			continue
		}
		cmd := exec.CommandContext(ctx, "bash", "-lc", c)
		if dir != "" {
			cmd.Dir = dir
		}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%q: %w: %s", c, err, strings.TrimSpace(stderr.String()))
		}
	}
	return nil
}

// effectiveWorkDir resolves a case's workdir. An empty workdir stays empty; an
// absolute workdir is used as-is; a relative workdir resolves against the case
// file's directory so artifacts land beside the case (e.g. cases/out/...). When
// the case has no Path (e.g. synthesized in tests), the relative value is used
// unchanged.
func effectiveWorkDir(c cases.Case) string {
	if c.WorkDir == "" {
		return ""
	}
	if filepath.IsAbs(c.WorkDir) {
		return c.WorkDir
	}
	if c.Path != "" {
		return filepath.Join(filepath.Dir(c.Path), c.WorkDir)
	}
	return c.WorkDir
}

// resolveSaveTo makes a relative save_to path relative to the effective workdir
// if set, else the case file's directory, so artifacts land where they belong.
func resolveSaveTo(c cases.Case, workDir, saveTo string) string {
	if filepath.IsAbs(saveTo) {
		return saveTo
	}
	if workDir != "" {
		return filepath.Join(workDir, saveTo)
	}
	if c.Path != "" {
		return filepath.Join(filepath.Dir(c.Path), saveTo)
	}
	return saveTo
}

// fencedBlock matches the contents of the first ``` fenced code block,
// ignoring an optional language tag like ```html.
var fencedBlock = regexp.MustCompile("(?s)```[a-zA-Z0-9]*\\n(.*?)```")

// extractCode returns the contents of the first fenced code block if the text
// has one; otherwise it returns the trimmed text unchanged. Models often wrap
// code in ```html ... ```, and we want the file to contain just the code.
func extractCode(text string) string {
	if m := fencedBlock.FindStringSubmatch(text); m != nil {
		return strings.TrimRight(m[1], "\n") + "\n"
	}
	return strings.TrimSpace(text) + "\n"
}
