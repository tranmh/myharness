package scorer

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// exact passes when the (trimmed) output equals the expected value.
type exact struct {
	want       string
	ignoreCase bool
}

func (s *exact) Score(_ context.Context, output string) Result {
	got := strings.TrimSpace(output)
	want := s.want
	if s.ignoreCase {
		got, want = strings.ToLower(got), strings.ToLower(want)
	}
	if got == want {
		return Result{Pass: true, Score: 1, Detail: fmt.Sprintf("output equals %q", s.want)}
	}
	return Result{Pass: false, Score: 0, Detail: fmt.Sprintf("output != %q", s.want)}
}

func (s *exact) Describe() string { return fmt.Sprintf("exact %q", s.want) }

// contains passes when the output contains the expected substring.
type contains struct {
	want       string
	ignoreCase bool
}

func (s *contains) Score(_ context.Context, output string) Result {
	hay, needle := output, s.want
	if s.ignoreCase {
		hay, needle = strings.ToLower(hay), strings.ToLower(needle)
	}
	if strings.Contains(hay, needle) {
		return Result{Pass: true, Score: 1, Detail: fmt.Sprintf("found %q", s.want)}
	}
	return Result{Pass: false, Score: 0, Detail: fmt.Sprintf("missing %q", s.want)}
}

func (s *contains) Describe() string { return fmt.Sprintf("contains %q", s.want) }

// regex passes when the pattern matches anywhere in the output.
type regex struct {
	re *regexp.Regexp
}

func newRegex(pattern string) (*regex, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	return &regex{re: re}, nil
}

func (s *regex) Score(_ context.Context, output string) Result {
	if s.re.MatchString(output) {
		return Result{Pass: true, Score: 1, Detail: fmt.Sprintf("matched /%s/", s.re.String())}
	}
	return Result{Pass: false, Score: 0, Detail: fmt.Sprintf("no match for /%s/", s.re.String())}
}

func (s *regex) Describe() string { return fmt.Sprintf("regex /%s/", s.re.String()) }

// judge uses a model to decide whether the output satisfies a rubric. It builds
// a verdict prompt embedding the rubric and the candidate output, asks the
// model (via JudgeFunc) to reply starting with PASS or FAIL: <reason>, and
// passes when the first token is PASS.
type judgeScorer struct {
	rubric string
	fn     JudgeFunc
}

func (s *judgeScorer) Score(ctx context.Context, output string) Result {
	if s.fn == nil {
		return Result{Pass: false, Score: 0, Detail: "no judge model available (judge scorer needs a provider)"}
	}

	prompt := fmt.Sprintf(`You are grading a candidate answer against a rubric.

RUBRIC:
%s

CANDIDATE ANSWER:
%s

Decide whether the candidate answer satisfies the rubric. Reply with a single
line that starts with PASS if it does, or FAIL: <short reason> if it does not.
Do not output anything else.`, strings.TrimSpace(s.rubric), output)

	reply, err := s.fn(ctx, prompt)
	if err != nil {
		return Result{Pass: false, Score: 0, Detail: fmt.Sprintf("judge call failed: %v", err)}
	}

	verdict := strings.TrimSpace(reply)
	first := verdict
	if idx := strings.IndexAny(verdict, " \t\n:"); idx >= 0 {
		first = verdict[:idx]
	}
	if strings.EqualFold(first, "PASS") {
		return Result{Pass: true, Score: 1, Detail: "judge: PASS"}
	}
	detail := verdict
	if detail == "" {
		detail = "judge returned an empty verdict"
	}
	return Result{Pass: false, Score: 0, Detail: "judge: " + detail}
}

func (s *judgeScorer) Describe() string { return "judge: " + strings.TrimSpace(s.rubric) }
