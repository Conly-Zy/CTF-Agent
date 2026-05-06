package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestDoneBarrierTool(t *testing.T) {
	ch := make(chan BarrierPayload, 1)
	tool := NewDoneBarrierTool(ch)

	// Test name and description
	if tool.Name() != "barrier_done" {
		t.Errorf("expected name 'barrier_done', got %q", tool.Name())
	}

	// Test schema
	schema := tool.Schema()
	if schema["type"] != "object" {
		t.Error("expected type 'object'")
	}

	// Test execute with valid input
	input := json.RawMessage(`{"flag": "flag{test}", "summary": "found flag"}`)
	go func() {
		output, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if output == "" {
			t.Error("expected non-empty output")
		}
	}()

	// Wait for barrier payload
	select {
	case payload := <-ch:
		if payload.Type != BarrierDone {
			t.Errorf("expected type 'done', got %q", payload.Type)
		}
		if payload.Flag != "flag{test}" {
			t.Errorf("expected flag 'flag{test}', got %q", payload.Flag)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for barrier payload")
	}
}

func TestDoneBarrierTool_MissingFlag(t *testing.T) {
	ch := make(chan BarrierPayload, 1)
	tool := NewDoneBarrierTool(ch)

	input := json.RawMessage(`{"summary": "no flag"}`)
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing flag")
	}
}

func TestAskBarrierTool(t *testing.T) {
	resultCh := make(chan BarrierPayload, 1)
	answersCh := make(chan string, 1)
	tool := NewAskBarrierTool(resultCh, answersCh)

	// Test name
	if tool.Name() != "barrier_ask" {
		t.Errorf("expected name 'barrier_ask', got %q", tool.Name())
	}

	// Test execute with valid input
	input := json.RawMessage(`{"question": "how to solve?", "context": "web challenge"}`)

	go func() {
		// Simulate answer
		answersCh <- "try SQL injection"
	}()

	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if output != "try SQL injection" {
		t.Errorf("expected 'try SQL injection', got %q", output)
	}
}
