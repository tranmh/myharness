# Case Library Run Results

Generated 2026-06-13 by executing the full 24-case demo library in `cases/`.

Two passes were run from the project root (`/home/tranmh/work/myharness`):

1. **Mock pass** — free, no API cost; proves the pipeline parses cases, runs
   scorers, and saves reports/artifacts end-to-end.
2. **Real `claude` pass** — calls the real `claude` CLI (costs money, authorized).

## Reproduce

```bash
cd /home/tranmh/work/myharness

# 1. Free mock pass
go run ./cmd/harness run cases/ --provider mock --verbose \
  --output results/mock.report.json 2>&1 | tee results/mock.console.txt

# 2. Real claude pass (cost-bounded; per-case budgets in the case files apply)
go run ./cmd/harness run cases/ --max-tries 2 --interactive never \
  --timeout 240 --parallel 4 --output results/run.report.json --verbose \
  2>&1 | tee results/run.console.txt
```

`--interactive never` is required so a failing case never blocks on stdin.
The harness exits non-zero if any case fails; that is expected and is not a
failure of the run itself.

## Summary

| Pass         | Provider | Passed | Failed | Total cost | Total duration |
|--------------|----------|--------|--------|-----------|----------------|
| Mock         | mock     | 7      | 17     | $0.0000   | ~0s            |
| Real claude  | claude   | 24     | 0      | $1.1069   | 304.41s (sum of per-case; wall-clock lower due to `--parallel 4`) |

### Mock pass (7 passed, 17 failed)

The mock provider returns canned, case-aware output. For 7 cases that canned
output happens to satisfy the scorers; for the other 17 it does not. This is
expected — the point of the mock pass is to prove the parse -> run -> score ->
save pipeline works without spending money, not to actually pass the cases.

- Passed (7): build-tetris, capital-of-france, code-review, haiku-poem,
  simple-arithmetic, summarize, tetris-phases
- Failed (17): api-design-then-impl, bash-oneliner, calculator-html,
  csv-to-json, fizzbuzz, json-extract, logic-puzzle, markdown-table,
  refactor-then-test, regex-author, sentiment, slugify, snake-game, sql-query,
  translate-fr, unit-convert, word-problem

### Real claude pass (24 passed, 0 failed)

All 24 cases ran against the real `claude` CLI and **all passed**. Total cost
**$1.1069**, total $\sum$ per-case duration **304.41s** (~5 min). Wall-clock was
shorter because the run used `--parallel 4`. No cases failed, so there are no
failure reasons to report.

## Per-case results (real run)

Cost and duration are read from `results/run.report.json` (durations were
nanoseconds in the JSON, shown here in seconds).

| Case                  | Result | Cost     | Duration | Tags                      |
|-----------------------|--------|----------|----------|---------------------------|
| api-design-then-impl  | PASS   | $0.0953  | 10.0s    | phases,code,judge         |
| bash-oneliner         | PASS   | $0.0166  | 4.2s     | code                      |
| build-tetris          | PASS   | $0.1033  | 50.3s    | (none)                    |
| calculator-html       | PASS   | $0.0810  | 29.1s    | html,judge                |
| capital-of-france     | PASS   | $0.0058  | 1.9s     | (none)                    |
| code-review           | PASS   | $0.0501  | 8.3s     | judge,code                |
| csv-to-json           | PASS   | $0.0182  | 5.1s     | data                      |
| fizzbuzz              | PASS   | $0.0186  | 3.6s     | code                      |
| haiku-poem            | PASS   | $0.0174  | 3.0s     | judge                     |
| json-extract          | PASS   | $0.0163  | 2.8s     | data                      |
| logic-puzzle          | PASS   | $0.0449  | 3.8s     | reasoning                 |
| markdown-table        | PASS   | $0.0166  | 2.5s     | data                      |
| refactor-then-test    | PASS   | $0.0967  | 8.2s     | phases,code               |
| regex-author          | PASS   | $0.0192  | 7.0s     | code                      |
| sentiment             | PASS   | $0.0371  | 2.6s     | qa,reasoning              |
| simple-arithmetic     | PASS   | $0.0446  | 1.9s     | (none)                    |
| slugify               | PASS   | $0.0165  | 3.0s     | code,data                 |
| snake-game            | PASS   | $0.0764  | 26.7s    | game,html                 |
| sql-query             | PASS   | $0.0179  | 5.1s     | code,data                 |
| summarize             | PASS   | $0.0167  | 2.6s     | judge                     |
| tetris-phases         | PASS   | $0.2487  | 114.1s   | game,html,phases,judge    |
| translate-fr          | PASS   | $0.0166  | 4.3s     | qa,judge                  |
| unit-convert          | PASS   | $0.0163  | 2.2s     | qa                        |
| word-problem          | PASS   | $0.0162  | 2.1s     | reasoning                 |

## Generated artifacts (from `save_to` / `workdir` cases)

`save_to` paths resolve relative to the case's `workdir` (relative to the project
root), or to the case file's directory when no `workdir` is set. All expected
artifacts were produced:

| Case          | Path                                                          | Size  |
|---------------|---------------------------------------------------------------|-------|
| build-tetris  | `/home/tranmh/work/myharness/cases/tetris.html`               | 9464 B  |
| tetris-phases | `/home/tranmh/work/myharness/out/tetris/tetris.html`          | 10141 B |
| snake-game    | `/home/tranmh/work/myharness/out/snake/snake.html`            | 4731 B  |
| calculator-html | `/home/tranmh/work/myharness/out/calc/calc.html`            | 6927 B  |
| refactor-then-test | `/home/tranmh/work/myharness/out/refactor/is_palindrome.py`      | 99 B    |
| refactor-then-test | `/home/tranmh/work/myharness/out/refactor/test_is_palindrome.py` | 1251 B  |

Note: the workdir-based cases (snake, calc, tetris-phases) write under
`out/<name>/` at the **project root**, not `cases/out/`. The standalone
`build-tetris` case has no `workdir` so it writes next to its case file at
`cases/tetris.html`.

## Failures

None. The real `claude` run passed all 24 cases on the first or second try
(`--max-tries 2`). No case-level failures, timeouts, or budget overruns.

## Multi-file project cases (real run — INTERRUPTED)

A second real-`claude` run was started for the 10 multi-file, multi-phase project
cases (selected with `--tag multi-file`), after the `workdir` fix that makes
relative `workdir`/`save_to` resolve under the **case file's directory** (so these
artifacts correctly land under `cases/out/<case>/`):

```bash
go run ./cmd/harness run cases/ --tag multi-file --max-tries 2 \
  --interactive never --timeout 300 --parallel 4 \
  --output results/multifile.report.json --verbose 2>&1 \
  | tee results/multifile.console.txt
```

That first batch run was manually stopped before it finished (the 10-phase
`full-snake-game-project` was the bottleneck). It was then **re-run per-case in
parallel sub-agents** — one sub-agent per case, each invoking the harness on a
single case file — skipping `full-snake-game-project`:

```bash
# one of these per case (run independently, in parallel sub-agents)
go run ./cmd/harness run cases/<case>.json --max-tries 2 --interactive never \
  --timeout 300 --output results/<case>.report.json --verbose \
  2>&1 | tee results/<case>.console.txt
```

**Result: all 10 cases PASS** (every phase passed on the first try except where
noted). Total cost of the passing runs ≈ **$2.88**.

| Case                       | Phases | Result | Files   | Cost     | Duration |
|----------------------------|--------|--------|---------|----------|----------|
| `static-landing-page`      | 3      | PASS   | 3 / 3   | $0.2290  | 35.5s    |
| `python-package`           | 3      | PASS   | 3 / 3   | $0.1833  | 14.0s    |
| `go-cli-tool`              | 4      | PASS   | 4 / 4   | $0.2331  | 12.0s    |
| `flask-todo-api`           | 5      | PASS   | 5 / 5   | $0.3607  | 42.6s    |
| `c-linked-list`            | 4      | PASS   | 4 / 4   | $0.2376  | 19.3s    |
| `bash-backup-toolkit`      | 2      | PASS*  | 2 / 2   | $0.1187  | 5.8s     |
| `sql-schema-and-seed`      | 3      | PASS   | 3 / 3   | $0.2319  | 27.4s    |
| `react-counter-spa`        | 4      | PASS   | 4 / 4   | $0.2514  | 23.5s    |
| `dockerized-node-app`      | 6      | PASS   | 6 / 6   | $0.3538  | 23.4s    |
| `full-snake-game-project`  | 10     | PASS   | 10 / 10 | $0.6781  | 70.4s    |

All judge scorers (on the integration/README phase of several cases) returned
PASS. **44 files** were generated across the 10 cases, one per phase. The 10-phase
`full-snake-game-project` was run **solo** in its own sub-agent (it finished in
~70s on its own; under the original `--parallel 4` batch it was starved and became
the bottleneck).

\* `bash-backup-toolkit` failed on the first attempt due to a **case-authoring
bug**, not a model error: its shebang scorer `^#!/usr/bin/env bash` used a
start-of-string anchor, but scorers run on the **raw** model output, which begins
with the ```` ```bash ```` code fence — so `^` never reached the shebang on line 2.
Fixed by adding the `(?m)` multiline flag (`(?m)^#!/usr/bin/env bash`); the re-run
then passed. (Lesson: `^`/`$`-anchored regex scorers need `(?m)` because the model
wraps output in a fenced block and scorers see the fence.)

Generated files live under `cases/out/<case>/` and are **committed to the repo**
as reference output (see below).

## Saved outputs

Every case's outputs are tracked in git as reference examples:

- **Generated files** — `cases/out/<case>/` (44 files from the 10 multi-file
  cases, plus earlier single-file outputs: `calc/`, `snake/`, `tetris/`,
  `refactor/`) and `cases/tetris.html` (the `build-tetris` case).
- **Per-case text outputs** — every case's full model output, per phase/attempt,
  is in the `*.report.json` files (the `output` fields). `run.report.json` holds
  the original 24-case real run (covers the text-only QA/code cases); the
  per-case `<case>.report.json` files hold the 10 multi-file re-runs.
- **Human-readable logs** — the matching `*.console.txt` files.

Only the mock-provider output (`mock.*`, canned, not real model output) and the
empty interrupted-batch stub (`multifile.*`) are git-ignored.

## Files in this directory

- `run.report.json` / `run.console.txt` — original 24-case real run (**tracked**)
- `<case>.report.json` / `<case>.console.txt` — per-case re-runs of the 10
  multi-file cases (**tracked**)
- `mock.report.json` / `mock.console.txt` — mock pass output (git-ignored, canned)
- `multifile.report.json` / `multifile.console.txt` — interrupted batch run stub
  (git-ignored; report.json was never written because the run was stopped early)
- `README.md` — this summary (tracked)
