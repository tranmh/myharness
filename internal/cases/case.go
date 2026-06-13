// Package cases defines what a test "case" looks like and how to load cases
// from JSON files on disk. A case is the unit of work the harness runs: a
// pipeline of one or more phases (each a prompt sent to the model plus the
// rules used to judge the response).
package cases

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScorerSpec is the JSON form of a scorer. The "type" field selects which
// scorer to build; the remaining fields are read by that scorer. Keeping all
// possible fields on one struct keeps the JSON simple for new users.
type ScorerSpec struct {
	Type       string `json:"type"`                  // "exact" | "contains" | "regex" | "judge"
	Value      string `json:"value,omitempty"`       // exact/contains: the expected text
	Pattern    string `json:"pattern,omitempty"`     // regex: the pattern
	IgnoreCase bool   `json:"ignore_case,omitempty"` // exact/contains: case-insensitive match
	Rubric     string `json:"rubric,omitempty"`      // judge: the rubric the output must satisfy
	JudgeModel string `json:"judge_model,omitempty"` // judge: optional model override for the verdict call
}

// Phase is one step in a case pipeline: a single prompt sent to the model plus
// the scorers that decide whether the phase passed. Phases run in order; a
// later phase can reference earlier phases' output via templates in its prompt.
type Phase struct {
	Name         string       `json:"name,omitempty"`
	Prompt       string       `json:"prompt"`
	Model        string       `json:"model,omitempty"`         // "", "opus", "sonnet", "haiku"
	System       string       `json:"system,omitempty"`        // optional system prompt
	AllowedTools []string     `json:"allowed_tools,omitempty"` // e.g. ["Write","Bash"]
	MaxTurns     int          `json:"max_turns,omitempty"`
	SaveTo       string       `json:"save_to,omitempty"`   // write the response to this file after the phase passes
	MaxTries     int          `json:"max_tries,omitempty"` // override the run-wide retry budget for this phase
	Scorers      []ScorerSpec `json:"scorers"`
}

// Case is a pipeline of phases plus run-wide configuration. It mirrors the JSON
// file format. The legacy single-prompt form (top-level prompt/scorers/etc.) is
// still accepted and normalized into a single phase by normalize().
type Case struct {
	Name string `json:"name"`

	// Phases is the full, multi-step form. When empty, the legacy top-level
	// fields below are used to synthesize a single phase.
	Phases []Phase `json:"phases,omitempty"`

	// Run-wide knobs.
	Tags          []string `json:"tags,omitempty"`
	WorkDir       string   `json:"workdir,omitempty"`         // cwd for setup/teardown and the provider
	Setup         []string `json:"setup,omitempty"`           // shell commands run before any phase
	Teardown      []string `json:"teardown,omitempty"`        // shell commands always run after all phases
	MaxCostUSD    float64  `json:"max_cost_usd,omitempty"`    // 0 means no cap
	MaxTurnsTotal int      `json:"max_turns_total,omitempty"` // 0 means no cap

	// Legacy single-prompt fields. Kept for backward compatibility; when no
	// phases are given these become a single phase via normalize().
	Prompt       string       `json:"prompt,omitempty"`
	Model        string       `json:"model,omitempty"`
	System       string       `json:"system,omitempty"`
	AllowedTools []string     `json:"allowed_tools,omitempty"`
	MaxTurns     int          `json:"max_turns,omitempty"`
	SaveTo       string       `json:"save_to,omitempty"`
	Scorers      []ScorerSpec `json:"scorers,omitempty"`

	// Path is the file the case was loaded from. Not part of the JSON; set by
	// the loader so reports and SaveTo can resolve relative paths.
	Path string `json:"-"`
}

// normalize folds the legacy single-prompt form into Phases, propagates
// case-level defaults into phases that leave them unset, and names any unnamed
// phase phase-1, phase-2, … It is called by the loader before Validate.
func (c *Case) normalize() {
	if len(c.Phases) == 0 && c.Prompt != "" {
		c.Phases = []Phase{{
			Prompt:       c.Prompt,
			Model:        c.Model,
			System:       c.System,
			AllowedTools: c.AllowedTools,
			MaxTurns:     c.MaxTurns,
			SaveTo:       c.SaveTo,
			Scorers:      c.Scorers,
		}}
	}

	for i := range c.Phases {
		p := &c.Phases[i]
		if p.Model == "" {
			p.Model = c.Model
		}
		if p.System == "" {
			p.System = c.System
		}
		if len(p.AllowedTools) == 0 {
			p.AllowedTools = c.AllowedTools
		}
		if p.MaxTurns == 0 {
			p.MaxTurns = c.MaxTurns
		}
		if strings.TrimSpace(p.Name) == "" {
			p.Name = fmt.Sprintf("phase-%d", i+1)
		}
	}
}

// Validate checks that a case is well-formed before we spend any tokens on it.
func (c Case) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("missing %q", "name")
	}
	if len(c.Phases) == 0 {
		return fmt.Errorf("case %q: needs at least one phase (or a top-level %q)", c.Name, "prompt")
	}
	for pi, p := range c.Phases {
		if strings.TrimSpace(p.Prompt) == "" {
			return fmt.Errorf("case %q phase %q: missing %q", c.Name, p.Name, "prompt")
		}
		if len(p.Scorers) == 0 {
			return fmt.Errorf("case %q phase %q: needs at least one scorer", c.Name, p.Name)
		}
		for i, s := range p.Scorers {
			if err := validateScorer(s); err != nil {
				return fmt.Errorf("case %q phase %q scorer #%d: %w", c.Name, p.Name, i+1, err)
			}
		}
		_ = pi
	}
	return nil
}

func validateScorer(s ScorerSpec) error {
	switch s.Type {
	case "exact", "contains":
		if s.Value == "" {
			return fmt.Errorf("(%s): missing %q", s.Type, "value")
		}
	case "regex":
		if s.Pattern == "" {
			return fmt.Errorf("(regex): missing %q", "pattern")
		}
	case "judge":
		if strings.TrimSpace(s.Rubric) == "" {
			return fmt.Errorf("(judge): missing %q", "rubric")
		}
	case "":
		return fmt.Errorf("missing %q", "type")
	default:
		return fmt.Errorf("unknown type %q", s.Type)
	}
	return nil
}

// Load reads cases from a path that is either a single .json file or a
// directory containing .json files (non-recursive). Cases are returned sorted
// by name so runs are deterministic.
func Load(path string) ([]Case, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var files []string
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				files = append(files, filepath.Join(path, e.Name()))
			}
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("no .json case files found in %s", path)
		}
	} else {
		files = []string{path}
	}

	var out []Case
	for _, f := range files {
		c, err := loadFile(f)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func loadFile(path string) (Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Case{}, err
	}
	var c Case
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields() // catch typos in case files early
	if err := dec.Decode(&c); err != nil {
		return Case{}, fmt.Errorf("%s: %w", path, err)
	}
	c.Path = path
	c.normalize()
	if err := c.Validate(); err != nil {
		return Case{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}
