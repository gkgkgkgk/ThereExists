// Package llm is a thin wrapper around an LLM chat API. Phase 5 uses it
// for civilization generation: one creative call for the description and
// name, one constrained-choice call for the tech profile.
//
// The package is a leaf — it does not import factory. Callers (civgen,
// handlers) compose the two.
package llm

import (
	"context"
	"errors"
	"time"
)

// Client is the interface every LLM backend implements. Tests inject a
// FakeClient; production wires an OpenAIClient.
type Client interface {
	// Complete returns a single text response to the prompt. Used for
	// open-ended creative generation.
	Complete(ctx context.Context, prompt string, opts ...Option) (string, error)

	// CompleteJSON prompts with a JSON schema and unmarshals the response
	// into out. The schema is an inline JSON-schema string (not a typed
	// struct) so callers can sprintf option menus into it without
	// ceremony.
	CompleteJSON(ctx context.Context, prompt string, schema string, out any, opts ...Option) error
}

// callConfig holds the tunables applied per call via Option. Defaults
// differ between Complete (creative) and CompleteJSON (structured) —
// each entry point seeds its own defaults before applying Options.
type callConfig struct {
	Model       string
	Temperature float64
	MaxTokens   int
	Timeout     time.Duration
}

// Option mutates a callConfig. Unknown options compose cleanly.
type Option func(*callConfig)

func WithModel(m string) Option            { return func(c *callConfig) { c.Model = m } }
func WithTemperature(t float64) Option     { return func(c *callConfig) { c.Temperature = t } }
func WithMaxTokens(n int) Option           { return func(c *callConfig) { c.MaxTokens = n } }
func WithTimeout(d time.Duration) Option   { return func(c *callConfig) { c.Timeout = d } }

// Sentinel errors. Callers distinguish transient (retry-worthy) from
// validation (bad output, caller must decide whether to retry with a
// corrective prompt).
var (
	ErrTransient  = errors.New("llm: transient failure")
	ErrValidation = errors.New("llm: response failed validation")
)

// applyDefaults returns a callConfig seeded with the given defaults and
// then mutated by opts.
func applyDefaults(defaults callConfig, opts []Option) callConfig {
	c := defaults
	for _, o := range opts {
		o(&c)
	}
	return c
}
