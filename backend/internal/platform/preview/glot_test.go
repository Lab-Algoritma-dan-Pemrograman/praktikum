package preview

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(server *httptest.Server) *Client {
	client := New(server.URL+"/api/mux", "fixture-token")
	client.http = server.Client()
	return client
}

func TestUnconfiguredClientNeverCallsProvider(t *testing.T) {
	client := New("https://glot.invalid/api/mux", "")
	if client.Enabled() {
		t.Fatal("client without token must be disabled")
	}
	if _, err := client.Run(context.Background(), "c", "int main(void){return 0;}", ""); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestRunSendsBoundedContractAndMapsSuccess(t *testing.T) {
	var received requestPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/mux" {
			t.Errorf("unexpected route %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token fixture-token" {
			t.Errorf("missing provider token header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content type %q", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(providerResult{Duration: 1, Stdout: "ok\n"})
	}))
	defer server.Close()

	result, err := testClient(server).Run(context.Background(), "c", "int main(void){puts(\"ok\");return 0;}", "in\n")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusSuccess || result.Stdout != "ok\n" {
		t.Fatalf("unexpected result %+v", result)
	}
	if received.Action != "run" || received.Data.Image != "glot/clang:latest" {
		t.Fatalf("unexpected envelope %+v", received)
	}
	if len(received.Data.Payload.Files) != 1 || received.Data.Payload.Files[0].Name != "main.c" {
		t.Fatalf("unexpected files %+v", received.Data.Payload.Files)
	}
	if received.Data.Payload.Stdin != "in\n" {
		t.Fatalf("stdin not forwarded")
	}
	if len(received.Data.Payload.RunInstructions.BuildCommands) != 1 ||
		received.Data.Payload.RunInstructions.RunCommand != "./a.out" {
		t.Fatalf("unexpected instructions %+v", received.Data.Payload.RunInstructions)
	}
}

func TestPythonUsesPythonImageAndRunCommand(t *testing.T) {
	var received requestPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		_ = json.NewEncoder(w).Encode(providerResult{Stdout: "1\n"})
	}))
	defer server.Close()
	if _, err := testClient(server).Run(context.Background(), "python", "print(1)", ""); err != nil {
		t.Fatal(err)
	}
	if received.Data.Image != "glot/python:latest" || received.Data.Payload.Files[0].Name != "main.py" ||
		received.Data.Payload.RunInstructions.RunCommand != "python main.py" ||
		len(received.Data.Payload.RunInstructions.BuildCommands) != 0 {
		t.Fatalf("unexpected python payload %+v", received)
	}
}

func TestErrorFieldIsRuntimeErrorWithoutInventingPhase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(providerResult{Stderr: "main.c:1: error", Error: "Exit code: 1"})
	}))
	defer server.Close()
	result, err := testClient(server).Run(context.Background(), "c", "int main(", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusRuntimeError {
		t.Fatalf("want runtime_error, got %s", result.Status)
	}
	if !strings.Contains(result.Stderr, "main.c:1: error") || !strings.Contains(result.Stderr, "Exit code: 1") {
		t.Fatalf("diagnostics lost: %q", result.Stderr)
	}
}

func TestMessageMapsToProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(providerResult{Message: "container unavailable"})
	}))
	defer server.Close()
	result, err := testClient(server).Run(context.Background(), "c", "int main(void){return 0;}", "")
	if err != nil || result.Status != StatusProviderError || !strings.Contains(result.Stderr, "container unavailable") {
		t.Fatalf("unexpected result %+v err %v", result, err)
	}
}

func TestProviderFailureIsNotRetried(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "upstream failure", http.StatusInternalServerError)
	}))
	defer server.Close()
	if _, err := testClient(server).Run(context.Background(), "c", "int main(void){return 0;}", ""); !errors.Is(err, ErrProvider) {
		t.Fatalf("want ErrProvider, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one provider call, got %d", calls)
	}
}

func TestOversizedProviderBodyIsRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", maxProviderBody+1)))
	}))
	defer server.Close()
	if _, err := testClient(server).Run(context.Background(), "c", "int main(void){return 0;}", ""); !errors.Is(err, ErrProvider) {
		t.Fatalf("want ErrProvider, got %v", err)
	}
}

func TestTimeoutMapsToTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(providerResult{Stdout: "late"})
	}))
	defer server.Close()
	client := testClient(server)
	client.http.Timeout = 30 * time.Millisecond
	if _, err := client.Run(context.Background(), "c", "int main(void){return 0;}", ""); !errors.Is(err, ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
}

func TestOutputIsBoundedAndFlagged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(providerResult{Stdout: strings.Repeat("x", 20_000), Stderr: strings.Repeat("y", 20_000)})
	}))
	defer server.Close()
	result, err := testClient(server).Run(context.Background(), "c", "int main(void){return 0;}", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Stdout) != 16384 || len(result.Stderr) != 16384 || !result.Truncated {
		t.Fatalf("unbounded output: stdout=%d stderr=%d truncated=%v", len(result.Stdout), len(result.Stderr), result.Truncated)
	}
}

func TestUnsupportedLanguageIsRejectedBeforeNetwork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("provider must not be called")
	}))
	defer server.Close()
	if _, err := testClient(server).Run(context.Background(), "ruby", "puts 1", ""); err == nil {
		t.Fatal("unsupported language was accepted")
	}
}
