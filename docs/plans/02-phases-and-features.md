# Plan 02 — Multi-phase cases, retries-with-feedback, and essential next features

## Context

The harness today runs a **single one-shot prompt per case**: `Case` has one
`prompt` + `scorers` + optional `save_to`, and `runner.runOne` does exactly one
provider call. That's enough for trivia/"build one file" evals but not for real
work, which is usually *staged* ("design, then implement", "build it, then add a
feature") and *flaky* (one shot often misses; you want the model to see what it
got wrong and try again).

This change adds:
1. **Phases** — a case is a *pipeline of prompts*. Each phase is its own fresh
   `claude` call with its own scorers; a phase only advances once its scorers
   pass. Later phases can reference earlier phases' output via templates.
2. **Retries with feedback** — each phase retries up to `max_tries` (default 5,
   configurable). On retry the model is re-sent the prompt **plus a note listing
   which scorers failed**, so it can self-correct.
3. **Interactive give-up** — after `max_tries` fails, when attached to a
   terminal, prompt: **[r]etry / [s]kip phase / [a]bort case / [q]uit run**.
   In CI / no-TTY this is non-interactive and the phase simply fails.
4. Four requested "essential missing" features: **LLM-as-judge scorer**,
   **per-case budgets**, **tags + filtering**, **setup/teardown + workdir**.

Design principles unchanged: small, readable, **zero external dependencies**
(stdlib only), clear seams, and **backward compatible** — every existing
single-prompt case keeps working untouched.

## New mental model

```
case ─► [setup] ─► phase 1 ─► phase 2 ─► ... ─► [teardown]
                     │
                     └─ attempt 1..N: provider call ─► scorers
                          ├─ all pass ─► phase done, output saved for later phases
                          ├─ fail, tries left ─► retry with failure feedback appended
                          └─ out of tries ─► give-up: retry/skip/abort/quit (or fail in CI)
   budget (cost/turns) accumulates across every phase + attempt; exceed ⇒ case fails
```

## Case file format (backward compatible)

New full form:

```json
{
  "name": "tetris-then-pause",
  "tags": ["game", "html"],
  "workdir": "out/tetris",
  "setup": ["mkdir -p ."],
  "teardown": [],
  "max_cost_usd": 1.0,
  "max_turns_total": 30,
  "model": "sonnet",
  "phases": [
    {
      "name": "build",
      "prompt": "Create a single-file HTML5 Tetris ... Output ONLY one ```html block.",
      "save_to": "tetris.html",
      "max_tries": 5,
      "scorers": [
        { "type": "contains", "value": "<canvas", "ignore_case": true },
        { "type": "regex", "pattern": "(?i)keydown|ArrowLeft" }
      ]
    },
    {
      "name": "pause",
      "prompt": "Here is the current game:\n{{.Prev}}\nAdd a pause button (key P). Output ONLY the full updated ```html block.",
      "save_to": "tetris.html",
      "scorers": [
        { "type": "contains", "value": "<canvas", "ignore_case": true },
        { "type": "judge", "rubric": "The HTML implements a working pause toggle on the 'p' key." }
      ]
    }
  ]
}
```

- **Legacy form still valid**: if `phases` is absent but top-level `prompt` is
  present, the loader synthesizes a single phase from the existing
  `prompt/model/system/allowed_tools/max_turns/save_to/scorers` fields. No
  existing case file changes.
- **Phase inherits** case-level `model`/`system`/`allowed_tools`/`max_turns`
  when its own are unset.
- **Templates**: phase prompts are run through Go `text/template` (stdlib) with
  `.Prev` (previous phase's final output) and `.Out` (a `map[string]string` of
  prior phase-name → output). No template markers ⇒ prompt used verbatim.

## Files to change

### `internal/cases/case.go` — phases, new fields, normalization
- Add `Phase` struct: `Name, Prompt, Model, System, AllowedTools, MaxTurns,
  SaveTo, MaxTries int, Scorers []ScorerSpec`.
- Extend `Case` with: `Phases []Phase`, `Tags []string`, `WorkDir string`,
  `Setup []string`, `Teardown []string`, `MaxCostUSD float64`,
  `MaxTurnsTotal int`. Keep the legacy `Prompt/Model/System/AllowedTools/
  MaxTurns/SaveTo/Scorers` fields.
- Extend `ScorerSpec` with `Rubric string` (and optional `JudgeModel string`)
  for the judge scorer.
- New `normalize()` called by `loadFile`: if `len(Phases)==0 && Prompt!=""`,
  build one `Phase` from legacy fields; then propagate case-level model/system/
  tools/max_turns defaults into phases that don't set them. Keep
  `DisallowUnknownFields` (new fields are now known).
- Rework `Validate()` to validate each phase (name optional → default
  `phase-1`…; prompt required; ≥1 scorer; existing scorer-field checks, plus
  `judge` requires `rubric`).

### `internal/scorer/scorer.go` + `builtin.go` — judge scorer, ctx-aware interface
- Change interface to `Score(ctx context.Context, output string) Result`
  (deterministic scorers ignore ctx). Update `exact/contains/regex`.
- Add `type JudgeFunc func(ctx context.Context, prompt string) (string, error)`.
- `Build(spec, judge JudgeFunc)` / `BuildAll(specs, judge)` thread the judge fn;
  add `judge` type in `builtin.go`: builds a rubric prompt embedding the
  candidate output, asks for a `PASS`/`FAIL: reason` verdict, parses the first
  token; `Describe()` = `judge: <rubric>`. Deterministic builders ignore
  `judge`.
- Update `scorer_test.go` for the new signature (pass `context.Background()`,
  nil judge) and add a judge test using a stub `JudgeFunc`.

### `internal/provider/provider.go` + `claude.go` — workdir
- Add `WorkDir string` to `Request`.
- In `claude.go`: when `WorkDir != ""`, set `cmd.Dir = WorkDir` and append
  `--add-dir`, WorkDir so file-producing cases operate inside their workdir.

### `internal/runner/runner.go` — the core rewrite
- New result types: `AttemptResult{Output, Scores []ScoreResult, Pass, CostUSD,
  NumTurns, Duration, Err}`, `PhaseResult{Name, Pass, Tries, Attempts
  []AttemptResult, SavedTo, Decision}`, and reshape `CaseResult` to
  `{Name, Tags, Pass, Phases []PhaseResult, CostUSD, NumTurns, Duration, Err}`
  (totals summed across phases/attempts).
- `Options` gains: `MaxTries int` (default 5), `Prompter Prompter`,
  `Judge JudgeFunc` (built from the provider in `main.go`).
- `runOne`: run `Setup` (bash) → for each phase run `runPhase` → always run
  `Teardown`. Stop early on abort/quit/budget-exceeded. Track cumulative cost &
  turns; if `MaxCostUSD`/`MaxTurnsTotal` exceeded after any call, fail the case.
- `runPhase`: loop up to `tries`; build prompt via `renderPrompt` (template with
  `.Prev`/`.Out`) and, on attempts >1, append `feedback(failedScorers)`; call
  provider; run scorers (`scorer.BuildAll(..., opts.Judge)`); on all-pass save
  via existing `extractCode`/`resolveSaveTo` and return; on exhaustion call
  `opts.Prompter.OnGiveUp(...)` → `Retry` (fresh try budget) / `Skip` (phase
  fails, continue) / `AbortCase` / `QuitRun` (cancel run ctx).
- New `Prompter` interface + `Decision` enum. Implementations:
  `terminalPrompter` (mutex-guarded `os.Stdin` line read — serializes prompts
  under `--parallel`), and a non-interactive default returning `Skip`/fail. A
  scripted prompter is used in tests (no TTY needed).
- `feedback()` builds: "Your previous attempt failed these checks:\n- <Describe>:
  <Detail>…\nPlease fix and output the full result again."
- Keep `resolveSaveTo` (resolve relative to `WorkDir` if set, else case dir) and
  `extractCode`/`fencedBlock` as-is.

### `cmd/harness/main.go` — flags, judge wiring, interactivity, filtering
- New `run` flags: `--max-tries` (5), `--interactive` (`auto|always|never`,
  default `auto` → TTY-detect via `os.Stdin.Stat()` `ModeCharDevice`),
  `--tag` (comma list, run cases matching ANY tag), `--name` (substring filter).
- Build `Judge JudgeFunc` from the chosen provider (calls `p.Run` with the
  phase/judge model) and pass it + the selected `Prompter` into `runner.Options`.
- Filter loaded cases by `--tag`/`--name` before running.
- Extend mock canned responses so `--provider mock` still demos cleanly
  (best-effort: key the new phases example's prompts; note in README that mock
  is best-effort for templated multi-phase prompts).

### `internal/report/report.go` — nested rendering
- `Console`: per case print overall PASS/FAIL + cost/turns/duration, then one
  indented line per phase (`name`, `tries`, pass, `saved:`). `--verbose` expands
  each attempt's scorers and truncated output, and shows the give-up decision.
- `JSON`: serialize the new nested `CaseResult`. Keep `truncate`. Still returns
  failed-case count for the exit code.

### Examples + docs
- Add `cases/tetris-phases.json` (build → add pause, with a `judge` scorer and a
  `workdir`) as the multi-phase demo. Leave `capital/math/tetris` untouched to
  prove backward compatibility.
- `README.md`: add a "Phases & retries" section (pipeline diagram, the
  feedback-retry loop, the interactive give-up menu + CI behavior), document
  `tags`, `workdir`, `setup`/`teardown`, `max_cost_usd`, `max_turns_total`, the
  `judge` scorer, and the new flags; move the now-implemented items out of the
  "deliberately left out" list.
- `.gitignore`: add `cases/out/` (workdir artifacts).

## Verification

- `go build ./... && go vet ./... && go test ./...` — scorer + runner tests pass
  with **no API calls** (runner test uses the Mock provider, a scripted
  Prompter, and a stub JudgeFunc).
- New/updated unit tests:
  - judge scorer with a stub `JudgeFunc` (PASS and FAIL verdicts);
  - a multi-phase case that fails phase 1's scorers on attempt 1 then passes on
    attempt 2 (assert the retry got feedback appended);
  - give-up `Skip`/`AbortCase` via scripted prompter;
  - budget exceeded ⇒ case fails;
  - legacy single-prompt case still normalizes to one phase and passes.
- `go run ./cmd/harness validate cases/` — all cases (legacy + new) parse, zero
  cost.
- `go run ./cmd/harness run cases/ --provider mock --verbose` — green, instant,
  no tokens.
- Real end-to-end: `go run ./cmd/harness run cases/tetris-phases.json --verbose`
  — expect phase `build` then `pause` to pass (with retries if needed), a
  generated `out/tetris/tetris.html`, and printed per-phase cost/turns.
- Interactive check: force a failure (e.g. an impossible scorer) on a TTY and
  confirm the `[r]/[s]/[a]/[q]` menu appears; run the same with
  `--interactive never` and confirm it fails without prompting.

## Demo case library (≥20 varied cases)

Add a `cases/` library that shows the tool's breadth. Each case is tagged so
`--tag` slices work. Cheap, deterministic cases use `haiku`; generative ones use
`sonnet`. Mix every scorer type (exact/contains/regex/judge) and every feature
(phases, budgets, workdir, setup/teardown). Representative set (keep the
existing `capital`/`math`/`tetris` and add the rest):

1. `capital` — factual QA, contains *(exists)*
2. `math` — arithmetic, regex *(exists)*
3. `tetris` — single-file HTML game, contains+regex, save_to *(exists)*
4. `tetris-phases` — **phases** (build → add pause), judge + workdir
5. `fizzbuzz` — code gen, exact-ish via regex/contains
6. `json-extract` — return strict JSON, regex validates shape
7. `csv-to-json` — data transform, contains keys
8. `sql-query` — generate SQL, contains `SELECT`/`JOIN`, regex
9. `regex-author` — produce a regex, contains anchors
10. `translate-fr` — translation, contains expected token + judge fluency
11. `summarize` — **judge**-scored summary quality, budget capped
12. `haiku` — **judge** 5-7-5 structure/quality
13. `sentiment` — classification, exact label
14. `logic-puzzle` — reasoning, contains the answer
15. `word-problem` — math reasoning, regex number
16. `markdown-table` — formatting, regex pipe rows
17. `bash-oneliner` — CLI, contains command + regex
18. `snake-game` — second HTML game, save_to, contains `<canvas>`
19. `calculator-html` — HTML/CSS/JS component, save_to + judge
20. `api-design-then-impl` — **phases**: design JSON spec → implement handler
     referencing `{{.Prev}}`, judge
21. `refactor-then-test` — **phases**: write function → write tests for it,
     workdir + setup
22. `code-review` — **judge** reviews a snippet against a rubric
23. `unit-convert` — deterministic exact
24. `slugify` — string transform, exact

Group with tags like `qa`, `code`, `html`, `game`, `phases`, `judge`, `data`,
`reasoning` so `--tag game` or `--tag judge` runs a themed subset.

## Execute and save results

After implementation + cases land, run the library and persist artifacts under
`results/` (gitignored except a committed README/sample):

- `go run ./cmd/harness run cases/ --provider mock --output results/mock.report.json --verbose`
  → free full pass (proves all 24 parse/score/save end-to-end); tee console to
  `results/mock.console.txt`.
- Real run, cost-bounded (small `--max-tries`, per-case budgets in the files):
  `go run ./cmd/harness run cases/ --output results/run.report.json --verbose`
  teed to `results/run.console.txt`. Generated artifacts (tetris/snake/
  calculator HTML, etc.) land in their case `workdir`s under `cases/out/`.
- Write `results/README.md` summarizing pass/fail counts, total cost, and how to
  reproduce.

## Save plans to docs/plans

Copy this project's plan(s) into a tracked `docs/plans/` folder so they live with
the repo: copy `/home/tranmh/.claude/plans/silly-whistling-sparrow.md` to
`docs/plans/02-phases-and-features.md`, and reconstruct the original v1 plan as
`docs/plans/01-initial-harness.md` (from the README/code it describes). Add
`docs/plans/README.md` indexing them.

## Execution strategy — sub-agents

Per the request, carry out the build with `general-purpose` sub-agents (the main
loop stays the coordinator and runs nothing destructive itself beyond wiring):

- **Agent A — core feature**: implement `cases`, `scorer` (judge + ctx),
  `provider` (workdir), `runner` (phases/retries/prompter/budgets), `report`,
  `cmd` flags; run `go build/vet/test` until green.
- **Agent B — case library**: author the ≥20 case JSON files + tags once Agent A's
  schema is in (depends on A; run after).
- **Agent C — execute & results**: run mock + real passes, save reports/console/
  artifacts, write `results/README.md` (depends on A+B).
- **Agent D — docs/plans**: create `docs/plans/` and the index (independent; can
  run in parallel with A).

Agents A and D run in parallel first; then B; then C. The coordinator reviews
each agent's diff/output before launching dependents.

## Out of scope (future)

Multi-turn session continuation (`--resume`), YAML cases, rate-limiting/backoff,
HTML report, parallel-safe interactive output (prompts are serialized via a
mutex for now).
