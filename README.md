# myharness

A tiny **LLM eval harness** written in Go. You describe *cases* (a prompt plus
how to judge the answer) in small JSON files; `myharness` runs them through the
[`claude` CLI](https://docs.claude.com/en/docs/claude-code) and prints a
pass/fail report with cost and timing.

It's the smallest useful version of the tooling teams use for **prompt
regression testing** and **evals**: write a check once, run it as often as you
like, and notice when behavior changes.

```
cases/*.json ─► Runner ─► Provider (claude -p --output-format json) ─► Scorers ─► Report
   inputs       loop +         the LLM call (text + cost + turns)       judge    console
            concurrency                                                          + JSON
```

A case can be a single prompt or a **pipeline of phases**, and each phase
**retries with feedback** until its scorers pass. See
[Phases & retries](#phases--retries).

## Requirements

- **Go 1.22+**
- The **`claude` CLI** installed and authenticated (`claude --version`). The
  harness shells out to it, so there are no API keys to manage here. You can
  also run everything with `--provider mock` (no CLI, no cost).

## Quick start

```sh
# 1. parse the example cases without calling the model (free)
go run ./cmd/harness validate cases/

# 2. run them all against the real claude CLI
go run ./cmd/harness run cases/ --verbose

# 3. or run them with the built-in mock provider (instant, no cost)
go run ./cmd/harness run cases/ --provider mock --verbose
```

`cases/` ships **30+ example cases** showcasing the harness's breadth — factual
QA, code generation, two HTML games, data transforms (CSV/JSON/SQL),
judge-scored quality (summaries, haiku, code review), reasoning, classification,
multi-phase pipelines, and multi-file project generation. They're tagged, so you
can run a themed slice:

```sh
go run ./cmd/harness run cases/ --tag game        # just the HTML games
go run ./cmd/harness run cases/ --tag judge        # just LLM-as-judge cases
go run ./cmd/harness run cases/ --tag multi-file   # just multi-file projects
```

Tags in use: `qa`, `code`, `data`, `reasoning`, `judge`, `html`, `web`, `game`,
`phases`, `multi-file`, `project`, plus language tags (`go`, `python`, `c`,
`sql`, `bash`, `react`, `node`, `docker`).

## Build a Tetris game in one command (the demo)

The harness isn't only for trivia scoring — a case can ask the model to *build*
something and save the result. `cases/tetris.json` asks Claude for a complete,
single-file HTML5 Tetris game, checks the output really looks like a game, and
writes it to disk:

```json
{
  "name": "build-tetris",
  "prompt": "Create a complete, single-file HTML5 Tetris game ... Output ONLY the full HTML file inside one ```html code block.",
  "model": "sonnet",
  "save_to": "tetris.html",
  "scorers": [
    { "type": "contains", "value": "<canvas", "ignore_case": true },
    { "type": "contains", "value": "<!DOCTYPE html", "ignore_case": true },
    { "type": "regex",    "pattern": "(?i)keydown|ArrowLeft|keyCode" }
  ]
}
```

Run it:

```sh
go run ./cmd/harness run cases/tetris.json
```

```
running 1 case(s) via claude provider...

PASS  build-tetris              51.54s  $0.1085
      saved: cases/tetris.html

1 passed, 0 failed — total 51.54s, $0.1085
```

Now open the artifact in a browser and play:

```sh
xdg-open cases/tetris.html      # macOS: open cases/tetris.html
```

What just happened, end to end:
- **prompt** → sent through the `claude` CLI,
- **scorers** → confirmed the response contains a `<canvas>`, a doctype, and
  key handling (so a blank/garbage answer would FAIL),
- **`save_to`** → the harness pulled the HTML out of the ```` ```html ```` code
  block and wrote `tetris.html` next to the case file.

Want it instantly and for free? `--provider mock` returns a bundled playable
Tetris so you can see the whole flow without spending tokens:

```sh
go run ./cmd/harness run cases/tetris.json --provider mock
```

## Phases & retries

Real work is usually *staged* ("design, then implement") and *flaky* (one shot
often misses). A case can be a **pipeline of phases**: each phase is its own
fresh `claude` call with its own scorers, and a phase only advances once its
scorers pass.

```
case ─► [setup] ─► phase 1 ─► phase 2 ─► ... ─► [teardown]
                     │
                     └─ attempt 1..N: provider call ─► scorers
                          ├─ all pass ─► phase done, output saved for later phases
                          ├─ fail, tries left ─► retry with failure feedback appended
                          └─ out of tries ─► give-up: retry/skip/abort/quit (or fail in CI)
   budget (cost/turns) accumulates across every phase + attempt; exceed ⇒ case fails
```

```json
{
  "name": "tetris-then-pause",
  "tags": ["game", "html", "phases"],
  "workdir": "out/tetris",
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
        { "type": "contains", "value": "<canvas", "ignore_case": true }
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

See `cases/tetris-phases.json` for the full, runnable version.

### Multi-file projects

Because each phase writes one file via its own `save_to`, a multi-phase case
naturally generates a **multi-file project**: one file per phase, all landing in
the shared `workdir`. Later phases pull earlier files into their prompt with
`{{.Out.<phase>}}` so the pieces stay consistent (the HTML, its CSS, and its JS
all agree).

`cases/` ships ~10 such project cases spanning 2 to 10 files — e.g.
`static-landing-page` (html/css/js), `python-package`, `go-cli-tool`,
`flask-todo-api`, `c-linked-list`, `dockerized-node-app`, and the fully
decomposed `full-snake-game-project` (10 files). Run one and inspect the output:

```sh
go run ./cmd/harness run cases/static-landing-page.json
ls cases/out/static-landing-page/        # index.html  styles.css  app.js
```

**Phase fields:** `name` (defaults to `phase-1`, `phase-2`, …), `prompt`,
`scorers`, plus optional `model`/`system`/`allowed_tools`/`max_turns` (inherited
from the case when unset), `save_to`, and `max_tries` (overrides `--max-tries`
for this phase).

**Templates.** Phase prompts run through Go `text/template`. A later phase can
reference earlier output with `{{.Prev}}` (the previous phase's final output) or
`{{.Out.build}}` (a map of phase name → output). A prompt with no `{{` markers
is used verbatim.

**Retries with feedback.** Each phase retries up to `max_tries` (default 5).
After a failed attempt the model is re-sent the prompt **plus a note listing
which scorers failed**, so it can self-correct.

**Give-up menu.** After `max_tries` fails, on a terminal the harness prompts:

```
[r]etry  [s]kip phase  [a]bort case  [q]uit run
```

- **retry** grants a fresh budget of tries; **skip** fails the phase but runs the
  rest of the case; **abort** stops this case; **quit** cancels the whole run.
- In CI / no-TTY (or `--interactive never`) there is no prompt: the phase simply
  fails and the run continues. `--interactive auto` (the default) detects a TTY;
  `--interactive always` forces the menu.

**Budgets, workdir, setup/teardown.** `max_cost_usd` / `max_turns_total` cap a
case's total cost/turns across all phases and attempts — exceeding either fails
the case. `workdir` is created if missing and becomes the cwd for `setup`,
`teardown` (both lists of shell commands), the model call, and relative
`save_to` paths. A **relative** `workdir` (and a relative `save_to`) resolves
**relative to the case file's directory**, not the process's working directory —
so a case in `cases/` with `"workdir": "out/tetris"` writes artifacts under
`cases/out/tetris/`. An absolute `workdir` is used as-is. `teardown` always runs,
even when a phase fails.

## Writing your own case

A case is one JSON file. The simplest possible one:

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

### Case fields

| field             | required | meaning                                                          |
|-------------------|----------|------------------------------------------------------------------|
| `name`            | yes      | unique label shown in the report                                 |
| `prompt`          | yes\*    | the user prompt (legacy single-phase form; \*or use `phases`)    |
| `scorers`         | yes\*    | list of checks; passes only if **all** pass (\*or per-phase)     |
| `phases`          | no       | multi-step pipeline (see [Phases & retries](#phases--retries))   |
| `model`           | no       | `sonnet` / `opus` / `haiku`, or a full model id (inherited by phases) |
| `system`          | no       | system prompt (inherited by phases)                              |
| `allowed_tools`   | no       | tools the model may use, e.g. `["Write"]` (inherited by phases)  |
| `max_turns`       | no       | cap on the model's tool-use turns (inherited by phases)          |
| `save_to`         | no       | write the response to this file (code block extracted if any); relative paths resolve against `workdir`, else the case file's dir |
| `tags`            | no       | labels for `--tag` filtering, e.g. `["game","html"]`             |
| `workdir`         | no       | working dir for the model call, `setup`/`teardown`, and `save_to`; relative paths resolve against the case file's dir (so artifacts land under `cases/out/...`) |
| `setup`           | no       | shell commands run (bash) before any phase                       |
| `teardown`        | no       | shell commands always run after all phases (even on failure)     |
| `max_cost_usd`    | no       | per-case budget cap; exceeding it fails the case                 |
| `max_turns_total` | no       | per-case turn budget across all phases/attempts                  |

A legacy single-prompt case is normalized into a one-phase pipeline
automatically, so existing case files keep working unchanged.

### Scorer types

| `type`     | fields                        | passes when…                                |
|------------|-------------------------------|---------------------------------------------|
| `exact`    | `value`, `ignore_case`        | trimmed output equals `value`               |
| `contains` | `value`, `ignore_case`        | output contains `value`                     |
| `regex`    | `pattern`                     | `pattern` matches anywhere in output        |
| `judge`    | `rubric`, `judge_model`       | a model judges the output against `rubric`  |

The **`judge`** scorer asks a model (via the same provider) to grade the output
against a rubric and reply `PASS` or `FAIL: <reason>`; the first token decides
pass/fail. With `--provider mock` (or no provider), judge scorers fail
gracefully with a clear message.

## CLI reference

```
harness run <path> [flags]     run a case file or a directory of .json cases
harness validate <path>        parse cases and report errors (no model calls)

run flags:
  --parallel N     concurrent cases (default 4)
  --model M        override the model for every case (e.g. sonnet, opus)
  --timeout S      per-call timeout in seconds (default 120)
  --max-tries N    retries per phase before giving up (default 5)
  --interactive M  give-up prompt: auto|always|never (default auto, TTY-detected)
  --tag T          only run cases with ANY of these tags (comma-separated)
  --name S         only run cases whose name contains S
  --output FILE    also write results as JSON (good for CI)
  --provider P     "claude" (default) or "mock"
  --verbose        print each scorer and the model output
```

`harness run` exits non-zero if any case fails, so it drops straight into CI.

## Project layout

```
cmd/harness/main.go          CLI: run / validate, flag parsing
internal/cases/              Case + ScorerSpec structs, JSON loader
internal/provider/           Provider interface; claude-CLI and mock impls
internal/scorer/             Scorer interface, registry, exact/contains/regex/judge
internal/runner/             worker pool, phases, retries, prompter, budgets
internal/report/             console summary + JSON output
cases/                       30+ example cases (QA, code, games, data, judge,
                             multi-phase pipelines, multi-file projects)
cases/out/                   generated artifacts from save_to/workdir (gitignored)
docs/plans/                  historical design docs
results/                     saved run reports + summary (README kept, reports ignored)
```

## Extending it

The two interfaces are the seams meant for growth:

- **New scorer** — implement `scorer.Scorer` (a `Score(ctx, output) Result` and
  `Describe()`) and register it in `scorer.Build`. The bundled `judge` scorer is
  an example that calls a model with a rubric and parses a `PASS`/`FAIL` verdict.
- **New provider** — implement `provider.Provider` to target a different
  backend (a raw HTTP API, a local model, …) without touching the runner.

Deliberately left out for now: multi-turn session continuation (`--resume`),
YAML cases, rate-limiting/backoff, and an HTML report. (LLM-as-judge, phases,
retries-with-feedback, budgets, tags, and setup/teardown/workdir are now built
in — see above.)
