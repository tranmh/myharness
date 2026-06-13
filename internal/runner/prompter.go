package runner

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// stdinMu serializes terminal prompts so that, under --parallel, only one case
// reads from os.Stdin (and prints its menu) at a time.
var stdinMu sync.Mutex

// NoninteractivePrompter is the default in CI / no-TTY: a give-up always skips
// the phase (which fails it) and lets the rest of the run continue.
type NoninteractivePrompter struct{}

// OnGiveUp always returns DecisionSkip.
func (NoninteractivePrompter) OnGiveUp(_, _ string, _ int, _ []string) Decision {
	return DecisionSkip
}

// TerminalPrompter asks the operator what to do via a menu printed to stderr,
// reading a single line from os.Stdin. It is safe under --parallel: prompts are
// serialized by a package-level mutex.
type TerminalPrompter struct{}

// OnGiveUp prints the failure context and the [r]/[s]/[a]/[q] menu, then reads a
// choice from stdin. Unknown input (or EOF) defaults to skip.
func (TerminalPrompter) OnGiveUp(caseName, phaseName string, tries int, failed []string) Decision {
	stdinMu.Lock()
	defer stdinMu.Unlock()

	fmt.Fprintf(os.Stderr, "\ncase %q phase %q failed after %d tr%s:\n",
		caseName, phaseName, tries, plural(tries))
	for _, f := range failed {
		fmt.Fprintf(os.Stderr, "  - %s\n", f)
	}
	fmt.Fprint(os.Stderr, "[r]etry  [s]kip phase  [a]bort case  [q]uit run > ")

	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "r", "retry":
		return DecisionRetry
	case "a", "abort":
		return DecisionAbortCase
	case "q", "quit":
		return DecisionQuitRun
	default: // "s", "skip", empty, EOF, anything else
		return DecisionSkip
	}
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
