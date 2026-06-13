// Command harness runs LLM eval cases through the `claude` CLI and reports
// pass/fail results.
//
// Usage:
//
//	harness run <path>        run cases (a .json file or a directory of them)
//	harness validate <path>   parse cases and report errors without running them
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"myharness/internal/cases"
	"myharness/internal/provider"
	"myharness/internal/report"
	"myharness/internal/runner"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "validate":
		os.Exit(cmdValidate(os.Args[2:]))
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

// mockTetrisHTML is a minimal but real, playable single-file Tetris used by the
// mock provider so the demo produces an openable tetris.html with no API calls.
const mockTetrisHTML = "```html\n" + `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Tetris (mock)</title>
<style>body{background:#111;color:#eee;font-family:sans-serif;text-align:center}
canvas{background:#000;border:2px solid #555;margin-top:10px}</style>
</head>
<body>
<h1>Tetris</h1>
<p>Score: <span id="score">0</span></p>
<canvas id="board" width="240" height="480"></canvas>
<script>
const c=document.getElementById('board'),x=c.getContext('2d'),S=24;
const COLS=10,ROWS=20,grid=Array.from({length:ROWS},()=>Array(COLS).fill(0));
const SHAPES=[[[1,1,1,1]],[[1,1],[1,1]],[[0,1,0],[1,1,1]],[[1,0,0],[1,1,1]],
 [[0,0,1],[1,1,1]],[[0,1,1],[1,1,0]],[[1,1,0],[0,1,1]]];
const COLORS=['#0ff','#ff0','#a0f','#00f','#f80','#0f0','#f00'];
let piece,px,py,pc,score=0;
function spawn(){const i=Math.floor(Math.random()*SHAPES.length);
 piece=SHAPES[i];pc=COLORS[i];px=3;py=0;if(hit(0,0,piece))alert('Game Over');}
function hit(dx,dy,p){for(let r=0;r<p.length;r++)for(let cc=0;cc<p[r].length;cc++)
 if(p[r][cc]){const nx=px+cc+dx,ny=py+r+dy;
 if(nx<0||nx>=COLS||ny>=ROWS||(ny>=0&&grid[ny][nx]))return true;}return false;}
function rotate(p){return p[0].map((_,i)=>p.map(row=>row[i]).reverse());}
function freeze(){piece.forEach((row,r)=>row.forEach((v,cc)=>{if(v)grid[py+r][px+cc]=pc;}));
 clearLines();spawn();}
function clearLines(){for(let r=ROWS-1;r>=0;r--){if(grid[r].every(v=>v)){
 grid.splice(r,1);grid.unshift(Array(COLS).fill(0));score+=100;
 document.getElementById('score').textContent=score;r++;}}}
function draw(){x.clearRect(0,0,c.width,c.height);
 grid.forEach((row,r)=>row.forEach((v,cc)=>{if(v){x.fillStyle=v;x.fillRect(cc*S,r*S,S-1,S-1);}}));
 if(piece)piece.forEach((row,r)=>row.forEach((v,cc)=>{if(v){x.fillStyle=pc;
 x.fillRect((px+cc)*S,(py+r)*S,S-1,S-1);}}));}
document.addEventListener('keydown',e=>{
 if(e.key==='ArrowLeft'&&!hit(-1,0,piece))px--;
 else if(e.key==='ArrowRight'&&!hit(1,0,piece))px++;
 else if(e.key==='ArrowDown'&&!hit(0,1,piece))py++;
 else if(e.key==='ArrowUp'){const r=rotate(piece);const sp=piece;piece=r;if(hit(0,0,piece))piece=sp;}
 draw();});
function tick(){if(!hit(0,1,piece))py++;else freeze();draw();}
spawn();draw();setInterval(tick,500);
</script>
</body>
</html>` + "\n```"

func usage() {
	fmt.Fprint(os.Stderr, `harness — a tiny LLM eval harness driven by the claude CLI

Usage:
  harness run <path> [flags]     run cases (a .json file or a directory)
  harness validate <path>        parse cases and report errors (no model calls)

Run flags:
  --parallel N     concurrent cases (default 4)
  --model M        override the model for every case (e.g. sonnet, opus)
  --timeout S      per-call timeout in seconds (default 120)
  --max-tries N    retries per phase before giving up (default 5)
  --interactive M  give-up prompt: auto|always|never (default auto, TTY-detected)
  --tag T          only run cases with ANY of these tags (comma-separated)
  --name S         only run cases whose name contains S
  --output FILE    also write results as JSON
  --provider P     "claude" (default) or "mock" (no API calls)
  --verbose        print each scorer and the model output
`)
}

// parseWithPositional parses flags that may appear before or after the single
// <path> argument. Go's flag package stops at the first non-flag token, so we
// pull the path out and re-parse whatever followed it.
func parseWithPositional(fs *flag.FlagSet, argv []string) (string, error) {
	var path string
	rest := argv
	for {
		if err := fs.Parse(rest); err != nil {
			return "", err
		}
		if fs.NArg() == 0 {
			break
		}
		if path != "" {
			return "", fmt.Errorf("unexpected extra argument %q", fs.Arg(0))
		}
		path = fs.Arg(0)
		rest = fs.Args()[1:]
	}
	if path == "" {
		return "", fmt.Errorf("expected exactly one <path>")
	}
	return path, nil
}

func cmdRun(argv []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	parallel := fs.Int("parallel", 4, "concurrent cases")
	model := fs.String("model", "", "override model for every case")
	timeout := fs.Int("timeout", 120, "per-call timeout in seconds")
	maxTries := fs.Int("max-tries", 5, "retries per phase before giving up")
	interactive := fs.String("interactive", "auto", "give-up prompt: auto|always|never")
	tag := fs.String("tag", "", "only run cases with ANY of these tags (comma-separated)")
	name := fs.String("name", "", "only run cases whose name contains this substring")
	output := fs.String("output", "", "write results as JSON to this file")
	provName := fs.String("provider", "claude", `provider: "claude" or "mock"`)
	verbose := fs.Bool("verbose", false, "print scorers and output")

	path, err := parseWithPositional(fs, argv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run: %v\n", err)
		return 2
	}

	cs, err := cases.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading cases: %v\n", err)
		return 1
	}

	cs = filterCases(cs, *tag, *name)
	if len(cs) == 0 {
		fmt.Fprintln(os.Stderr, "no cases matched the --tag/--name filters")
		return 1
	}

	var p provider.Provider
	switch *provName {
	case "claude":
		p = &provider.ClaudeCLI{}
	case "mock":
		// Canned answers for the bundled example cases so `--provider mock`
		// shows a realistic green run with no API calls. Unknown prompts fall
		// back to Default and will fail their scorers (as they should).
		cannedByName := map[string]string{
			"capital-of-france": "Paris",
			"simple-arithmetic": "The answer is 391.",
			"build-tetris":      mockTetrisHTML,
			"tetris-phases":     mockTetrisHTML, // build phase of the phases demo
		}
		responses := map[string]string{}
		for _, c := range cs {
			for _, ph := range c.Phases {
				if text, ok := cannedByName[c.Name]; ok {
					responses[ph.Prompt] = text
				}
			}
		}
		// Substring-keyed answers for templated multi-phase prompts (whose full
		// text isn't known ahead of time) and for judge verdict calls. Mock is
		// best-effort here: it keeps the demo green without real tokens.
		contains := map[string]string{
			"Add a pause feature":                        mockTetrisHTML, // the "pause" phase
			"You are grading a candidate answer against": "PASS",         // judge verdicts
		}
		p = &provider.Mock{Default: "mock response", Responses: responses, Contains: contains}
	default:
		fmt.Fprintf(os.Stderr, "unknown provider %q\n", *provName)
		return 2
	}

	// Build a judge function from the chosen provider so judge scorers can ask
	// the model for a verdict.
	judge := func(ctx context.Context, prompt string) (string, error) {
		resp, err := p.Run(ctx, provider.Request{Prompt: prompt, Model: *model})
		if err != nil {
			return "", err
		}
		return resp.Text, nil
	}

	prompter := selectPrompter(*interactive)

	fmt.Fprintf(os.Stderr, "running %d case(s) via %s provider...\n\n", len(cs), *provName)
	results := runner.Run(context.Background(), p, cs, runner.Options{
		Parallel:      *parallel,
		ModelOverride: *model,
		Timeout:       time.Duration(*timeout) * time.Second,
		MaxTries:      *maxTries,
		Prompter:      prompter,
		Judge:         judge,
	})

	failed := report.Console(os.Stdout, results, *verbose)

	if *output != "" {
		if err := report.JSON(*output, results); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", *output, err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "\nwrote %s\n", *output)
	}

	if failed > 0 {
		return 1
	}
	return 0
}

// filterCases keeps cases matching the --tag (ANY of) and --name (substring)
// filters. Empty filters match everything.
func filterCases(cs []cases.Case, tagCSV, name string) []cases.Case {
	var tags []string
	for _, t := range strings.Split(tagCSV, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	if len(tags) == 0 && name == "" {
		return cs
	}
	var out []cases.Case
	for _, c := range cs {
		if name != "" && !strings.Contains(c.Name, name) {
			continue
		}
		if len(tags) > 0 && !anyTag(c.Tags, tags) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func anyTag(have, want []string) bool {
	for _, w := range want {
		for _, h := range have {
			if h == w {
				return true
			}
		}
	}
	return false
}

// selectPrompter resolves the --interactive mode to a Prompter. auto detects a
// TTY on stdin; always forces the terminal menu; never is non-interactive.
func selectPrompter(mode string) runner.Prompter {
	switch mode {
	case "always":
		return runner.TerminalPrompter{}
	case "never":
		return runner.NoninteractivePrompter{}
	default: // auto
		if isTerminal(os.Stdin) {
			return runner.TerminalPrompter{}
		}
		return runner.NoninteractivePrompter{}
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func cmdValidate(argv []string) int {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	path, err := parseWithPositional(fs, argv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate: %v\n", err)
		return 2
	}

	cs, err := cases.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "INVALID: %v\n", err)
		return 1
	}
	for _, c := range cs {
		scorers := 0
		for _, ph := range c.Phases {
			scorers += len(ph.Scorers)
		}
		fmt.Printf("ok  %-24s  %d phase(s)  %d scorer(s)  (%s)\n",
			c.Name, len(c.Phases), scorers, c.Path)
	}
	fmt.Printf("\n%d case(s) OK\n", len(cs))
	return 0
}
