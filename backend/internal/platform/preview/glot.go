// Package preview proxies the legacy Glot service for browser-preview fallback
// only. Results are never completion proof: the public codec exposes no
// trustworthy exit code, phase, or runtime identity.
package preview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/platform/runner"
)

const (
	StatusSuccess       = "success"
	StatusCompileError  = "compile_error"
	StatusRuntimeError  = "runtime_error"
	StatusTimeout       = "timeout"
	StatusProviderError = "provider_error"

	maxProviderBody = 256 << 10
	providerTimeout = 10 * time.Second
)

var (
	ErrNotConfigured = errors.New("playground fallback is not configured")
	ErrTimeout       = errors.New("playground fallback timed out")
	ErrProvider      = errors.New("playground fallback provider failed")
)

type Result struct {
	Status     string
	Stdout     string
	Stderr     string
	Truncated  bool
	DurationMS int64
}

type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

var languages = map[string]struct {
	image string
	file  string
	build string
	run   string
}{
	"c":      {image: "glot/clang:latest", file: "main.c", build: "clang -o a.out -lm main.c", run: "./a.out"},
	"python": {image: "glot/python:latest", file: "main.py", run: "python main.py"},
}

func New(endpoint, token string) *Client {
	return &Client{
		endpoint: endpoint,
		token:    token,
		http: &http.Client{
			Timeout: providerTimeout,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          4,
				MaxIdleConnsPerHost:   2,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: providerTimeout,
			},
		},
	}
}

func (c *Client) Enabled() bool { return c != nil && c.token != "" && c.endpoint != "" }

type filePayload struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type runPayload struct {
	RunInstructions struct {
		BuildCommands []string `json:"buildCommands"`
		RunCommand    string   `json:"runCommand"`
	} `json:"runInstructions"`
	Files []filePayload `json:"files"`
	Stdin string        `json:"stdin"`
}

type requestPayload struct {
	Action string `json:"action"`
	Data   struct {
		Image   string     `json:"image"`
		Payload runPayload `json:"payload"`
	} `json:"data"`
}

type providerResult struct {
	Duration int64  `json:"duration"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Error    string `json:"error"`
	Message  string `json:"message"`
}

// Run sends one bounded request. It never retries and never forwards provider
// bodies, tokens, or source text to callers or logs.
func (c *Client) Run(ctx context.Context, language, source, stdin string) (Result, error) {
	if !c.Enabled() {
		return Result{Status: StatusProviderError}, ErrNotConfigured
	}
	languageSpec, ok := languages[language]
	if !ok {
		return Result{Status: StatusProviderError}, runner.ErrUnsupportedLanguage
	}
	payload := requestPayload{Action: "run"}
	payload.Data.Image = languageSpec.image
	payload.Data.Payload.RunInstructions.BuildCommands = []string{}
	if languageSpec.build != "" {
		payload.Data.Payload.RunInstructions.BuildCommands = []string{languageSpec.build}
	}
	payload.Data.Payload.RunInstructions.RunCommand = languageSpec.run
	payload.Data.Payload.Files = []filePayload{{Name: languageSpec.file, Content: source}}
	payload.Data.Payload.Stdin = stdin
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{Status: StatusProviderError}, ErrProvider
	}

	endpoint, err := url.Parse(c.endpoint)
	if err != nil {
		return Result{Status: StatusProviderError}, ErrNotConfigured
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return Result{Status: StatusProviderError}, ErrProvider
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Token "+c.token)

	started := time.Now()
	response, err := c.http.Do(request)
	if err != nil {
		if isTimeout(err) {
			return Result{Status: StatusTimeout, DurationMS: elapsed(started)}, ErrTimeout
		}
		return Result{Status: StatusProviderError, DurationMS: elapsed(started)}, ErrProvider
	}
	defer response.Body.Close()

	limited := io.LimitReader(response.Body, maxProviderBody+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		if isTimeout(err) {
			return Result{Status: StatusTimeout, DurationMS: elapsed(started)}, ErrTimeout
		}
		return Result{Status: StatusProviderError, DurationMS: elapsed(started)}, ErrProvider
	}
	if len(raw) > maxProviderBody || response.StatusCode != http.StatusOK {
		return Result{Status: StatusProviderError, DurationMS: elapsed(started)}, ErrProvider
	}
	var decoded providerResult
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return Result{Status: StatusProviderError, DurationMS: elapsed(started)}, ErrProvider
	}
	result := Result{DurationMS: elapsed(started)}
	switch {
	case decoded.Message != "":
		result.Status = StatusProviderError
		result.Stderr = decoded.Message
	case decoded.Error != "":
		// The public codec cannot identify compile versus runtime phase.
		result.Status = StatusRuntimeError
		result.Stderr = decoded.Stderr + "\n" + decoded.Error
	default:
		result.Status = StatusSuccess
		result.Stderr = decoded.Stderr
	}
	var stdoutChanged bool
	result.Stdout, stdoutChanged = runner.BoundText(decoded.Stdout, runner.MaxOutputBytes)
	var stderrChanged bool
	result.Stderr, stderrChanged = runner.BoundText(result.Stderr, runner.MaxOutputBytes)
	result.Truncated = stdoutChanged || stderrChanged
	return result, nil
}

func elapsed(start time.Time) int64 {
	value := time.Since(start).Milliseconds()
	if value < 0 {
		return 0
	}
	if value > 30_000 {
		return 30_000
	}
	return value
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout()
}
