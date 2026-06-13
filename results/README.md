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

## Files in this directory

- `mock.report.json` / `mock.console.txt` — mock pass output (git-ignored)
- `run.report.json` / `run.console.txt` — real pass output (git-ignored)
- `README.md` — this summary (kept in git)
