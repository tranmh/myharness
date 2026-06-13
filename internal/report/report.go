// Package report renders run results: a human-readable summary to the console
// and, optionally, a machine-readable JSON file.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"myharness/internal/runner"
)

// Console writes a per-case summary plus totals to w. Each case prints its
// overall PASS/FAIL with cost/turns/duration, then one indented line per phase.
// When verbose, each phase also expands its attempts' scorers, truncated
// output, and any give-up decision. It returns the number of failed cases so
// the caller can set the process exit code.
func Console(w io.Writer, results []runner.CaseResult, verbose bool) int {
	var passed, failed int
	var totalCost float64
	var totalDur time.Duration

	for _, r := range results {
		status := "PASS"
		if !r.Pass {
			status = "FAIL"
			failed++
		} else {
			passed++
		}
		totalCost += r.CostUSD
		totalDur += r.Duration

		tags := ""
		if len(r.Tags) > 0 {
			tags = "  [" + strings.Join(r.Tags, ",") + "]"
		}
		fmt.Fprintf(w, "%s  %-24s  %5.2fs  $%.4f  %dt%s\n",
			status, r.Name, r.Duration.Seconds(), r.CostUSD, r.NumTurns, tags)
		if r.Err != "" {
			fmt.Fprintf(w, "      error: %s\n", r.Err)
		}

		for _, ph := range r.Phases {
			pmark := "ok"
			if !ph.Pass {
				pmark = "XX"
			}
			saved := ""
			if ph.SavedTo != "" {
				saved = "  saved:" + ph.SavedTo
			}
			dec := ""
			if ph.Decision != "" {
				dec = "  gave-up:" + ph.Decision
			}
			fmt.Fprintf(w, "    [%s] phase %-16s %d tr%s%s%s\n",
				pmark, ph.Name, ph.Tries, plural(ph.Tries), saved, dec)

			if verbose {
				for ai, a := range ph.Attempts {
					fmt.Fprintf(w, "        attempt %d:\n", ai+1)
					if a.Err != "" {
						fmt.Fprintf(w, "          error: %s\n", a.Err)
					}
					for _, s := range a.Scores {
						mark := "ok"
						if !s.Result.Pass {
							mark = "XX"
						}
						fmt.Fprintf(w, "          [%s] %s — %s\n", mark, s.Describe, s.Result.Detail)
					}
					if a.Output != "" {
						fmt.Fprintf(w, "          output: %s\n", truncate(a.Output, 280))
					}
				}
			}
		}
	}

	fmt.Fprintf(w, "\n%d passed, %d failed — total %.2fs, $%.4f\n",
		passed, failed, totalDur.Seconds(), totalCost)
	return failed
}

// JSON writes the full results to path as indented JSON.
func JSON(path string, results []runner.CaseResult) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
