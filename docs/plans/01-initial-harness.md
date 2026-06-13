# Plan 01 — A tiny LLM eval harness (v1)

## Goal

Build the **smallest useful version** of the tooling teams use for prompt
regression testing and evals: a way to write a check once — a prompt plus how to
judge the answer — and run it as often as you like, noticing when behavior
changes.

Concretely:
- A single **Go CLI** binary (`harness`).
- Driven by the **`claude` CLI** (`claude -p --output-format json`) rather than a
  raw HTTP API, so there are **no API keys to manage** in the tool itself — it
  shells out to an already-installed, already-authenticated `claude`.
- **Zero external dependencies**: stdlib only. Small, readable, easy to extend.
- Cases described as **small JSON files** so anyone can author one without
  touching Go.
- Output a **pass/fail report** with cost and timing, and exit non-zero on any
  failure so it drops straight into CI.

We deliberately keep v1 minimal. LLM-as-judge scoring, YAML cases, retries,
rate-limiting, multi-turn conversations, and an HTML report are explicitly out of
scope for this first version — but the design leaves clean seams for them.

## Mental model

```
cases/*.json ─► Runner ─► Provider (claude -p --output-format json) ─► Scorers ─► Report
   inputs       loop +         the LLM call (text + cost + turns)       judge    console
            concurrency                                                          + JSON
```

A **case** is the unit of work: one prompt sent to the model plus the rules used
to judge the response. The **runner** loops over cases concurrently, asks the
**provider** for an answer, runs every **scorer** over that answer, optionally
saves the answer to disk, and hands results to the **report** layer.

The two interfaces — `Provider` and `Scorer` — are the seams meant for growth.
You can add a new backend or a new kind of check without touching the runner.

## Packages

```
cmd/harness/main.go          CLI: run / validate, flag parsing
internal/cases/              Case + ScorerSpec structs, JSON loader
internal/provider/           Provider interface; claude-CLI and mock impls
internal/scorer/             Scorer interface, registry, exact/contains/regex
internal/runner/             worker pool, scoring, save_to + code extraction
internal/report/             console summary + JSON output
cases/                       example cases (capital, math, tetris)
```

## Case file format

A case is one JSON file. The loader (`cases.Load`) accepts either a single
`.json` file or a directory of them (non-recursive, `.json` only), and returns
the cases **sorted by name** for deterministic runs. Decoding uses
`DisallowUnknownFields` so typos in case files are caught early rather than
silently ignored.

The `Case` struct mirrors the JSON exactly:

| field           | required | meaning                                                       |
|-----------------|----------|---------------------------------------------------------------|
| `name`          | yes      | unique label shown in the report                              |
| `prompt`        | yes      | the user prompt sent to the model                             |
| `scorers`       | yes      | list of checks; the case PASSes only if **all** pass          |
| `model`         | no       | `sonnet` / `opus` / `haiku`, or a full model id               |
| `system`        | no       | system prompt                                                 |
| `allowed_tools` | no       | tools the model may use, e.g. `["Write"]`                     |
| `max_turns`     | no       | cap on the model's tool-use turns                             |
| `save_to`       | no       | write the response to this file (code block extracted if any) |

A `Path` field is set by the loader (not part of the JSON) so reports and
`save_to` can resolve paths relative to the case file's directory.

`Case.Validate()` runs before any tokens are spent: `name` and `prompt` must be
non-empty, there must be at least one scorer, and every scorer must have a known
`type` with its required fields present.

Simplest possible case:

```json
{
  "name": "capital-of-france",
  "prompt": "What is the capital of France? Answer with one word only.",
  "model": "sonnet",
  "system": "You are terse. Answer with a single word, no punctuation.",
  "scorers": [
    { "type": "contains", "value": "Paris", "ignore_case": true }
  ]
}
```

### ScorerSpec

Scorers are described by a single flat `ScorerSpec` struct (all possible fields
on one struct keeps the JSON simple). The `type` selects which scorer to build;
the rest are read by that scorer:

- `type` — `"exact" | "contains" | "regex"`
- `value` — exact/contains: the expected text
- `pattern` — regex: the pattern
- `ignore_case` — exact/contains: case-insensitive match

## The Provider seam

`provider.Provider` is the boundary between the harness and whatever actually
produces a response:

```go
type Provider interface {
    Run(ctx context.Context, r Request) (Response, error)
}
```

`Request` is one LLM call in provider-neutral terms: `Prompt`, `Model`,
`System`, `AllowedTools`, `MaxTurns`. `Response` carries the answer plus
reporting metadata: `Text`, `NumTurns`, `CostUSD`, `Duration`, `IsError`, and a
`Raw` payload for debugging.

### `ClaudeCLI` (default)

Shells out to the `claude` binary (default `"claude"`, overridable) in print
mode with JSON output. It builds the arg list from the request:
`-p <prompt> --output-format json`, plus `--model`, `--system-prompt`,
`--allowedTools` (comma-joined), and `--max-turns` when those fields are set.
The context controls timeout/cancellation. It decodes the JSON object printed by
`claude` (fields `result`, `is_error`, `num_turns`, `duration_ms`,
`total_cost_usd`, `session_id`) and maps it onto a `Response`. No API key
handling lives here — that is the `claude` CLI's job.

### `Mock`

A `Provider` that returns canned responses without calling anything, so the
runner and CLI can be exercised in tests and via `--provider mock` with no token
spend. It holds a `Responses map[string]string` keyed by prompt and a `Default`
string; an unknown prompt gets `Default` (and will fail its scorers, as it
should).

## The Scorer seam

`scorer.Scorer` is the boundary for "how do we judge an answer":

```go
type Scorer interface {
    Score(output string) Result   // pass/fail + score + human-readable detail
    Describe() string             // short label for reports, e.g. contains "Paris"
}
```

`Result` is `{Pass bool, Score float64, Detail string}` (score is 0..1; 1.0 ==
pass for the boolean scorers).

A single `Build(spec)` function maps a spec `type` to a concrete scorer — the
one place to add a new scorer type. `BuildAll(specs)` builds every scorer for a
case, failing fast on the first bad spec.

### Three builtin scorers

| `type`     | fields                 | passes when…                          |
|------------|------------------------|---------------------------------------|
| `exact`    | `value`, `ignore_case` | trimmed output equals `value`         |
| `contains` | `value`, `ignore_case` | output contains `value`               |
| `regex`    | `pattern`              | `pattern` matches anywhere in output  |

- **exact** trims the output and compares; lowercases both sides when
  `ignore_case`.
- **contains** does a substring test; lowercases both sides when `ignore_case`.
- **regex** compiles `pattern` with `regexp.Compile` at build time (so a bad
  pattern is a validation error) and matches anywhere in the output.

The natural next scorer is an **LLM-as-judge**: call the `claude` CLI with a
rubric and parse a verdict. The interface is shaped so that drops in without
touching the runner.

## The Runner

`runner.Run(ctx, provider, cases, opts)` ties the pieces together and returns
one `CaseResult` per case, in input order.

`Options`:
- `Parallel int` — max concurrent cases (default 4)
- `ModelOverride string` — if set, overrides every case's model
- `Timeout time.Duration` — per-case timeout (default 120s)

**Worker pool**: a buffered channel of size `Parallel` acts as a semaphore; each
case runs in its own goroutine, results written into a pre-sized slice by index
(so order is preserved without locking), joined by a `WaitGroup`.

`runOne` for a single case:
1. Resolve the model (case model, or the override).
2. Apply the per-case timeout via `context.WithTimeout`.
3. Call `provider.Run` with the request built from the case. On error, record it
   and stop.
4. Copy `Text`, `CostUSD`, `NumTurns`, `Duration` into the result. If the
   provider reports `IsError`, fail the case.
5. Build all scorers and run each over the output. The case **passes only if
   every scorer passes**; each scorer's describe + result is recorded.
6. If `save_to` is set, write the response to disk **regardless of pass/fail**
   (so you can inspect what was produced even when a scorer is unhappy).

`CaseResult` carries: `Name`, `Pass`, `Scores []ScoreResult`, `Output`,
`SavedTo`, `CostUSD`, `NumTurns`, `Duration`, `Err`.

### `save_to` + code-fence extraction

When a case asks for a file, models usually wrap code in a fenced block like
```` ```html … ``` ````. `extractCode` returns the contents of the **first**
fenced code block (a `(?s)```[a-zA-Z0-9]*\n(.*?)``` ` regex, ignoring the
optional language tag) when present, otherwise the trimmed text unchanged — so
the written file contains just the code, not the prose around it.
`resolveSaveTo` makes a relative `save_to` path relative to the case file's
directory, so artifacts land next to the case that produced them.

## The Report layer

`report.Console(w, results, verbose)` prints a per-case line
(`PASS/FAIL  name  duration  $cost`), any error, and totals
(`N passed, M failed — total Xs, $Y`). When `verbose`, it also prints each
scorer line (`[ok]/[XX] describe — detail`), the `saved:` path, and a truncated
copy of the model output. It **returns the number of failed cases** so the
caller can set the exit code.

`report.JSON(path, results)` writes the full results as indented JSON — good for
CI artifacts and diffing runs.

## CLI surface

```
harness run <path> [flags]     run a case file or a directory of .json cases
harness validate <path>        parse cases and report errors (no model calls)
```

`run` flags:

```
--parallel N     concurrent cases (default 4)
--model M        override the model for every case (e.g. sonnet, opus)
--timeout S      per-case timeout in seconds (default 120)
--output FILE    also write results as JSON (good for CI)
--provider P     "claude" (default) or "mock"
--verbose        print each scorer and the model output
```

Notes:
- Flags may appear **before or after** the single `<path>` argument; a small
  `parseWithPositional` helper re-parses around Go's flag-stops-at-first-nonflag
  behavior and rejects more than one positional.
- `--provider mock` wires a `Mock` with canned answers keyed by the bundled
  example prompts (capital → "Paris", arithmetic → an answer, build-tetris → a
  bundled playable Tetris), so the whole flow demos green, instantly, with no
  token spend. Unknown prompts get the default and fail their scorers.
- `harness run` exits non-zero if any case fails (drops straight into CI);
  `validate` parses every case and reports OK/INVALID without any model calls.

## The Tetris demo

The harness isn't only for trivia scoring — a case can ask the model to *build*
something and save the result. `cases/tetris.json` asks Claude for a complete,
single-file HTML5 Tetris game, scores that the output really looks like a game
(`contains "<canvas"`, `contains "<!DOCTYPE html"`, and a
`regex (?i)keydown|ArrowLeft|keyCode` for key handling), and uses `save_to:
"tetris.html"` to pull the HTML out of the ```` ```html ```` block and write it
next to the case file. So the end-to-end story is: prompt → `claude` →
scorers confirm it's a real game (a blank/garbage answer FAILs) → `save_to`
writes a playable artifact you can open in a browser.

For a free, instant version, `--provider mock` returns a bundled, genuinely
playable single-file Tetris (defined in `main.go`) so the full flow — including
code-fence extraction and `save_to` — can be seen without spending tokens.

## Example cases shipped

`cases/` ships three: `capital` (factual QA, contains), `math` (arithmetic,
regex), and `tetris` (single-file HTML game, contains + regex, `save_to`).

## Verification

- `go build ./... && go vet ./... && go test ./...` green; scorer and runner
  tests run with **no API calls** (the runner test uses the Mock provider).
- `go run ./cmd/harness validate cases/` — all example cases parse, zero cost.
- `go run ./cmd/harness run cases/ --provider mock --verbose` — green, instant,
  no tokens.
- `go run ./cmd/harness run cases/tetris.json` — real end-to-end: scorers pass
  and a playable `cases/tetris.html` is written.

## Deliberately out of scope (v1)

LLM-as-judge scoring, YAML cases, retries and rate-limiting, multi-turn
conversations, and an HTML report. The `Provider` and `Scorer` interfaces are
the intended growth seams for these.
