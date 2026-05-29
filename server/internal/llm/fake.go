package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

// FakeClient is a programmable Client for tests. Responses pop FIFO from
// the queued slices; errors pop in lockstep and short-circuit the call.
// Lives in the main package (no build tag) so consumers can construct it
// in tests without import gymnastics.
type FakeClient struct {
	CompleteResponses     []string
	CompleteJSONResponses []string
	CompleteErrs          []error
	CompleteJSONErrs      []error

	CompleteCalls     int
	CompleteJSONCalls int
}

func (f *FakeClient) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	f.CompleteCalls++
	if len(f.CompleteErrs) > 0 {
		err := f.CompleteErrs[0]
		f.CompleteErrs = f.CompleteErrs[1:]
		if err != nil {
			return "", err
		}
	}
	if len(f.CompleteResponses) == 0 {
		return "", fmt.Errorf("fake llm: no queued Complete response (call #%d)", f.CompleteCalls)
	}
	resp := f.CompleteResponses[0]
	f.CompleteResponses = f.CompleteResponses[1:]
	return resp, nil
}

func (f *FakeClient) CompleteJSON(ctx context.Context, prompt string, schema string, out any, opts ...Option) error {
	f.CompleteJSONCalls++
	if len(f.CompleteJSONErrs) > 0 {
		err := f.CompleteJSONErrs[0]
		f.CompleteJSONErrs = f.CompleteJSONErrs[1:]
		if err != nil {
			return err
		}
	}
	if len(f.CompleteJSONResponses) == 0 {
		return fmt.Errorf("fake llm: no queued CompleteJSON response (call #%d)", f.CompleteJSONCalls)
	}
	raw := f.CompleteJSONResponses[0]
	f.CompleteJSONResponses = f.CompleteJSONResponses[1:]
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return fmt.Errorf("%w: %v (raw: %s)", ErrValidation, err, raw)
	}
	return nil
}
