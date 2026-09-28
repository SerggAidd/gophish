package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientChatWithThinkingSendsEvaluatorSettings(t *testing.T) {
	var received chatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(chatResponse{
			Message:    Message{Content: `{"ok":true}`, Thinking: "short reasoning"},
			Done:       true,
			DoneReason: "stop",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "gpt-oss:20b")
	response, err := client.ChatWithThinking(
		context.Background(),
		[]Message{{Role: "user", Content: "test"}},
		map[string]interface{}{"type": "object"},
		evaluatorModelOptions(),
		evaluatorThinkLevel,
	)
	if err != nil {
		t.Fatalf("ChatWithThinking returned error: %v", err)
	}
	if response != `{"ok":true}` {
		t.Fatalf("unexpected response: %q", response)
	}
	if received.Think != evaluatorThinkLevel {
		t.Fatalf("expected think=%q, got %#v", evaluatorThinkLevel, received.Think)
	}
	if received.Options["temperature"] != float64(0) && received.Options["temperature"] != 0 {
		t.Fatalf("unexpected temperature: %#v", received.Options["temperature"])
	}
	if received.Options["seed"] != float64(evaluatorSeed) && received.Options["seed"] != evaluatorSeed {
		t.Fatalf("unexpected seed: %#v", received.Options["seed"])
	}
}

func TestClientRetriesHTTP500(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt == 1 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(chatResponse{
			Message:    Message{Content: "OK"},
			Done:       true,
			DoneReason: "stop",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test")
	response, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "test"}}, nil)
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if response != "OK" {
		t.Fatalf("unexpected response: %q", response)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

func TestClientRetriesEmptyContent(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		w.Header().Set("Content-Type", "application/json")

		if attempt == 1 {
			_ = json.NewEncoder(w).Encode(chatResponse{
				Message:    Message{Thinking: "reasoning without final answer"},
				Done:       true,
				DoneReason: "stop",
			})
			return
		}

		_ = json.NewEncoder(w).Encode(chatResponse{
			Message:    Message{Content: "OK"},
			Done:       true,
			DoneReason: "stop",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test")
	response, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "test"}}, nil)
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if response != "OK" {
		t.Fatalf("unexpected response: %q", response)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

func TestClientRetriesHTTPClientTimeout(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt == 1 {
			time.Sleep(60 * time.Millisecond)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(chatResponse{
			Message:    Message{Content: "OK"},
			Done:       true,
			DoneReason: "stop",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test")
	client.httpClient.Timeout = 20 * time.Millisecond

	response, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "test"}}, nil)
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if response != "OK" {
		t.Fatalf("unexpected response: %q", response)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

func TestClientDoesNotRetryCancelledContext(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(server.URL, "test")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Chat(ctx, []Message{{Role: "user", Content: "test"}}, nil)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if got := atomic.LoadInt32(&attempts); got != 0 {
		t.Fatalf("expected no HTTP attempt for already-cancelled context, got %d", got)
	}
}
