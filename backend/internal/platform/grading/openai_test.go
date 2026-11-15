package grading

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func jsonNew(r *http.Request) *json.Decoder { return json.NewDecoder(r.Body) }

func openaiServer(t *testing.T, status int, body string, seen *map[string]any, auth *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth != nil {
			*auth = r.Header.Get("Authorization")
		}
		if seen != nil {
			var payload map[string]any
			if err := jsonNew(r).Decode(&payload); err == nil {
				*seen = payload
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOpenAIGradeParsesSuggestions(t *testing.T) {
	var auth string
	var payload map[string]any
	srv := openaiServer(t, 200, `{"choices":[{"message":{"content":"{\"suggestions\":[{\"question_id\":\"q-1\",\"score\":7,\"feedback\":\"solid\"}]}","role":"assistant"}}]}`, &payload, &auth)
	adapter := NewOpenAI(srv.URL, "test-model", "test-key")
	result, err := adapter.Grade(context.Background(), validRequest())
	if err != nil || result.Status != StatusSuggested || len(result.Suggestions) != 1 || result.Suggestions[0].Score != 7 {
		t.Fatalf("grade = %+v, err %v", result, err)
	}
	if result.Simulated {
		t.Fatal("live adapter must never claim simulated")
	}
	if auth != "Bearer test-key" {
		t.Fatalf("auth header = %q", auth)
	}
	if payload["model"] != "test-model" {
		t.Fatalf("model = %v", payload["model"])
	}
}

func TestOpenAIGradeFallsBackToManualOnBadProvider(t *testing.T) {
	cases := map[string]*httptest.Server{
		"http500":   openaiServer(t, 500, `oops`, nil, nil),
		"badjson":   openaiServer(t, 200, `not json`, nil, nil),
		"nochoices": openaiServer(t, 200, `{"choices":[]}`, nil, nil),
		"garbled":   openaiServer(t, 200, `{"choices":[{"message":{"content":"here are my thoughts on your essay"}}]}`, nil, nil),
	}
	for name, srv := range cases {
		t.Run(name, func(t *testing.T) {
			adapter := NewOpenAI(srv.URL, "m", "")
			result, err := adapter.Grade(context.Background(), validRequest())
			if result.Status != StatusNeedsManual || len(result.Suggestions) != 0 || result.ErrorCode == "" {
				t.Fatalf("case %s: %+v", name, result)
			}
			if err == nil && name != "garbled" {
				t.Fatalf("case %s: expected error", name)
			}
		})
	}
}

func TestOpenAICompleteStripsFences(t *testing.T) {
	content := "```c\nint main(void){return 0;}\n```"
	body, err := json.Marshal(map[string]any{"choices": []map[string]any{{"message": map[string]any{"content": content, "role": "assistant"}}}})
	if err != nil {
		t.Fatal(err)
	}
	srv := openaiServer(t, 200, string(body), nil, nil)
	out, err := NewOpenAI(srv.URL, "m", "").Complete(context.Background(), "s", "u")
	if err != nil || strings.Contains(out, "```") || !strings.Contains(out, "int main") {
		t.Fatalf("complete = %q err %v", out, err)
	}
}

func TestOpenAICompletePropagatesHTTPError(t *testing.T) {
	srv := openaiServer(t, 503, ``, nil, nil)
	_, err := NewOpenAI(srv.URL, "m", "").Complete(context.Background(), "s", "u")
	if !errors.Is(err, ErrOpenAIBadResponse) {
		t.Fatalf("err = %v", err)
	}
}
