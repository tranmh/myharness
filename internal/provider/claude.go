package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ClaudeCLI runs prompts by invoking the `claude` command-line tool in print
// mode with JSON output. It relies on `claude` already being installed and
// authenticated on the machine — no API key handling lives here.
type ClaudeCLI struct {
	// Bin is the executable to run; defaults to "claude" when empty.
	Bin string
}

// claudeJSON mirrors the object printed by `claude -p --output-format json`.
// We only decode the fields we use.
type claudeJSON struct {
	Result     string  `json:"result"`
	IsError    bool    `json:"is_error"`
	NumTurns   int     `json:"num_turns"`
	DurationMs int64   `json:"duration_ms"`
	TotalCost  float64 `json:"total_cost_usd"`
	SessionID  string  `json:"session_id"`
}

// Run builds the claude command, executes it, and maps its JSON output onto a
// Response. The provided ctx controls the timeout/cancellation.
func (c *ClaudeCLI) Run(ctx context.Context, r Request) (Response, error) {
	bin := c.Bin
	if bin == "" {
		bin = "claude"
	}

	args := []string{"-p", r.Prompt, "--output-format", "json"}
	if r.Model != "" {
		args = append(args, "--model", r.Model)
	}
	if r.System != "" {
		args = append(args, "--system-prompt", r.System)
	}
	if len(r.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(r.AllowedTools, ","))
	}
	if r.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(r.MaxTurns))
	}
	if r.WorkDir != "" {
		args = append(args, "--add-dir", r.WorkDir)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	if r.WorkDir != "" {
		cmd.Dir = r.WorkDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Response{}, fmt.Errorf("claude timed out/cancelled: %w", ctx.Err())
		}
		return Response{}, fmt.Errorf("claude failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var parsed claudeJSON
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return Response{}, fmt.Errorf("could not parse claude JSON output: %w", err)
	}

	return Response{
		Text:     parsed.Result,
		NumTurns: parsed.NumTurns,
		CostUSD:  parsed.TotalCost,
		Duration: time.Duration(parsed.DurationMs) * time.Millisecond,
		IsError:  parsed.IsError,
		Raw:      json.RawMessage(stdout.Bytes()),
	}, nil
}
