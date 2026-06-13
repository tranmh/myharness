# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`myharness` is a tiny, zero-dependency Go LLM eval harness. Cases (a prompt + how to
judge the answer) are described in JSON files; the harness runs them through the
`claude` CLI and prints a pass/fail report with cost and timing. Standard library only —
do not add third-party dependencies without a strong reason.

## Commands

```sh
# Build / run the CLI
go run ./cmd/harness run <path> [flags]     # path = a .json case file or a dir of them
go run ./cmd/harness validate <path>        # parse + check cases, no model calls (free)

# Develop without spending tokens or needing the claude CLI:
go run ./cmd/harness run cases/ --provider mock --verbose

# Tests
go test ./...                                # all tests
go test ./internal/runner -run TestName      # a single test
go vet ./...
```

There is no lint config beyond `go vet`/`gofmt`. The test suite uses the `mock`
provider — it never shells out to `claude`, so `go test ./...` is hermetic and free.

## Architecture

The pipeline is a straight line, and each stage is a package under `internal/`:

```
cases/*.json ─► cases.Load ─► runner.Run ─► provider.Provider ─► scorer.Scorer ─► report
```

- **`internal/cases`** — `Case`/`Phase`/`ScorerSpec` structs and the JSON loader.
  Loader uses `DisallowUnknownFields` (typos in case files fail fast). `normalize()`
  folds the **legacy single-prompt form** (top-level `prompt`/`scorers`) into a
  one-element `Phases` slice and propagates case-level defaults (`model`, `system`,
  `allowed_tools`, `max_turns`) into phases that leave them unset. **Everything
  downstream only ever sees phases** — there is no single-prompt code path past the loader.
- **`internal/provider`** — `Provider` interface (`Run(ctx, Request) (Response, error)`).
  `ClaudeCLI` shells out to `claude -p <prompt> --output-format json` and decodes the
  result; `Mock` returns canned answers (keyed by exact prompt, then substring) for tests
  and the `--provider mock` demo. No API keys live here — auth is the `claude` CLI's job.
- **`internal/scorer`** — `Scorer` interface (`Score`/`Describe`) + the `Build` registry
  mapping a spec `type` to a concrete scorer. Built-ins: `exact`, `contains`, `regex`
  (deterministic) and `judge` (calls a model via `JudgeFunc`). A case passes a phase only
  when **all** its scorers pass.
- **`internal/runner`** — the orchestrator. A worker pool runs cases concurrently
  (`Options.Parallel`). Per case: `setup` shell commands → each phase in order → `teardown`
  (always runs, even on failure). Per phase: render the prompt → call provider → score →
  on failure **retry with feedback** (the failed scorer descriptions are appended to the
  prompt) up to `max_tries`, then ask the `Prompter` what to do.
- **`internal/report`** — console summary (returns failed count for the exit code) and
  optional JSON output.
- **`cmd/harness/main.go`** — flag parsing, provider/prompter selection, wires the `Judge`
  closure from the chosen provider into the runner. `parseWithPositional` lets flags appear
  before or after the `<path>` argument.

### Key cross-cutting behaviors

- **Phase templating**: phase prompts run through Go `text/template` with `.Prev`
  (previous phase's final output) and `.Out` (map of phase name → output). A prompt with
  no `{{` is used verbatim. This is how later phases reference earlier output.
- **`save_to`**: after a phase passes, the response is written to disk. `extractCode`
  pulls the contents of the first ```` ``` ```` fenced block if present, else the trimmed
  text. Relative paths resolve against `workdir`, else the case file's directory.
- **Budgets**: `max_cost_usd` / `max_turns_total` accumulate across *all* phases and
  attempts; exceeding either fails the whole case and stops it.
- **Give-up flow**: when a phase exhausts `max_tries`, the `Prompter` decides:
  retry (fresh budget) / skip (fail phase, continue case) / abort case / quit run.
  `--interactive auto` picks `TerminalPrompter` only when stdin is a TTY; CI/non-TTY gets
  `NoninteractivePrompter` (silently fails the phase and continues). `DecisionQuitRun`
  cancels the shared run context to stop other in-flight cases.

## Extending

The two interfaces are the intended seams:

- **New scorer**: implement `scorer.Scorer`, add a `case` to `scorer.Build`, and add any
  new JSON fields to `cases.ScorerSpec` + `validateScorer`.
- **New provider**: implement `provider.Provider`; wire it into the `switch` in
  `cmd/harness/main.go`'s `cmdRun`.

## The case library (`cases/`)

`cases/` holds 30+ example cases exercising every scorer type and feature, tagged for
`--tag` filtering (`qa`, `code`, `data`, `reasoning`, `judge`, `html`, `game`, `phases`,
`multi-file`, `project`, plus language tags). When adding cases:

- **Multi-file projects = one file per phase.** A phase writes exactly one file via its
  `save_to`; the harness saves only the **first** fenced block of the response, so each
  phase's prompt must end with "Output ONLY the contents of `<file>` in a single ```` ``` ````
  block." Later phases reference earlier files via `{{.Out.<phase>}}` for consistency.
- **Workdir is relative to the case file's dir**, so use `"workdir": "out/<case-name>"`; it
  resolves to `cases/out/<case-name>/`. That directory is gitignored.
- **Template field names must be valid Go identifiers** for `.Out.<phase>` to parse — name
  phases as single lowercase words (no hyphens), or use `{{index .Out "phase-name"}}`.
- Use `haiku` for cheap/deterministic cases, `sonnet` for generative/HTML/multi-phase ones.
- After editing cases, run `go run ./cmd/harness validate cases/` (must be clean) and a
  `--provider mock --interactive never` run (must not panic).

## Conventions

- Keep it **gofmt-clean** and `go vet`-clean; `go test ./...` must stay hermetic (mock
  provider, scripted prompter, stub judge — no real `claude` calls in tests).
- `docs/plans/` holds **historical** design docs (numbered, snapshots of intent). They may
  describe wording/features that have since changed — treat the source and `README.md` as
  the current truth, not the plans.
- `README.md` is the user-facing reference and is kept detailed/current; update it when you
  change case fields, scorer types, or CLI flags.
- `results/` holds saved run artifacts: `results/README.md` is committed; the bulky
  `*.report.json` / `*.console.txt` are gitignored.
