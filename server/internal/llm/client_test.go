package llm

import (
	"context"
	"errors"
	"testing"
)

func TestFakeClient_CompletePopsFIFO(t *testing.T) {
	f := &FakeClient{CompleteResponses: []string{"first", "second"}}
	ctx := context.Background()

	got, err := f.Complete(ctx, "ignored")
	if err != nil || got != "first" {
		t.Fatalf("call 1: got (%q, %v), want (\"first\", nil)", got, err)
	}
	got, err = f.Complete(ctx, "ignored")
	if err != nil || got != "second" {
		t.Fatalf("call 2: got (%q, %v), want (\"second\", nil)", got, err)
	}
	if f.CompleteCalls != 2 {
		t.Fatalf("CompleteCalls = %d, want 2", f.CompleteCalls)
	}
}

func TestFakeClient_CompleteErrorPassesThrough(t *testing.T) {
	boom := errors.New("boom")
	f := &FakeClient{CompleteErrs: []error{boom}}
	_, err := f.Complete(context.Background(), "ignored")
	if !errors.Is(err, boom) {
		t.Fatalf("got err %v, want %v", err, boom)
	}
}

func TestFakeClient_CompleteJSONUnmarshals(t *testing.T) {
	f := &FakeClient{CompleteJSONResponses: []string{`{"name":"Acme","tier":4}`}}
	var out struct {
		Name string `json:"name"`
		Tier int    `json:"tier"`
	}
	if err := f.CompleteJSON(context.Background(), "ignored", "{}", &out); err != nil {
		t.Fatalf("CompleteJSON: %v", err)
	}
	if out.Name != "Acme" || out.Tier != 4 {
		t.Fatalf("unmarshaled = %+v", out)
	}
}

func TestFakeClient_CompleteJSONInvalidJSON(t *testing.T) {
	f := &FakeClient{CompleteJSONResponses: []string{`not json`}}
	var out map[string]any
	err := f.CompleteJSON(context.Background(), "ignored", "{}", &out)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("got err %v, want ErrValidation", err)
	}
}

func TestFakeClient_ExhaustedQueueErrors(t *testing.T) {
	f := &FakeClient{}
	_, err := f.Complete(context.Background(), "ignored")
	if err == nil {
		t.Fatal("expected error on empty queue")
	}
}
