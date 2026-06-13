package provider

import (
	"context"
	"strings"
)

// Mock is a Provider that returns canned responses without calling anything.
// It lets the runner and CLI be exercised in tests (and with --provider mock)
// without spending tokens. Lookup order: an exact match in Responses, then the
// first substring in Contains found anywhere in the prompt, then Default. The
// Contains map handles templated multi-phase prompts (e.g. a "pause" phase
// whose prompt embeds the previous phase's output), where the full prompt text
// is not known ahead of time.
type Mock struct {
	Responses map[string]string
	Contains  map[string]string
	Default   string
}

func (m *Mock) Run(_ context.Context, r Request) (Response, error) {
	if v, ok := m.Responses[r.Prompt]; ok {
		return Response{Text: v, NumTurns: 1}, nil
	}
	for needle, v := range m.Contains {
		if strings.Contains(r.Prompt, needle) {
			return Response{Text: v, NumTurns: 1}, nil
		}
	}
	return Response{Text: m.Default, NumTurns: 1}, nil
}
