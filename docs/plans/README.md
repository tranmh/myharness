# Plans

This folder collects the design plans for `myharness`, kept in the repo so the
thinking behind the code lives alongside it. These are **historical design
documents**: snapshots of intent captured before (or alongside) the work, not
living API reference. They are numbered in the order they were taken on, and may
describe features or wording that has since evolved — read the source and
`README.md` for the current state.

## Index

- [`01-initial-harness.md`](01-initial-harness.md) — **The initial harness (v1).**
  A tiny LLM eval harness: a zero-dependency Go CLI driven by the `claude` CLI.
  Cases as JSON → runner → provider → scorers → report; the `Provider` and
  `Scorer` seams; the `exact`/`contains`/`regex` scorers; the worker-pool runner;
  `save_to` with code-fence extraction; the mock provider; the `run`/`validate`
  CLI; and the Tetris demo.
- [`02-phases-and-features.md`](02-phases-and-features.md) — **Multi-phase cases
  and essential next features.** Extends the harness with phase pipelines,
  retries-with-feedback, an interactive give-up menu, an LLM-as-judge scorer,
  per-case budgets, tags + filtering, and setup/teardown + workdir — all
  backward compatible with v1 single-prompt cases.
