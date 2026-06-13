// Package provider is the seam between the harness and whatever actually
// produces a response. The default provider shells out to the `claude` CLI,
// but anything implementing Provider (e.g. the mock used in tests) works.
package provider

import (
	"context"
	"encoding/json"
	"time"
)

// Request is one LLM call described in provider-neutral terms.
type Request struct {
	Prompt       string
	Model        string   // "", "opus", "sonnet", "haiku", or a full model id
	System       string   // optional system prompt
	AllowedTools []string // tools the model may use, e.g. ["Write"]
	MaxTurns     int      // 0 means provider default
	WorkDir      string   // working directory for the call; "" means the current dir
}

// Response is what a provider returns. Text is the answer; the rest is
// metadata useful for reporting (cost, how many tool turns it took, etc.).
type Response struct {
	Text     string          `json:"text"`
	NumTurns int             `json:"num_turns"`
	CostUSD  float64         `json:"cost_usd"`
	Duration time.Duration   `json:"duration"`
	IsError  bool            `json:"is_error"`
	Raw      json.RawMessage `json:"-"` // original provider payload, for debugging
}

// Provider runs a single request and returns the response.
type Provider interface {
	Run(ctx context.Context, r Request) (Response, error)
}
