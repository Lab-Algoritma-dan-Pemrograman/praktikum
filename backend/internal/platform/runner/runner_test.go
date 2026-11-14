package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

func validRequest() entity.RunRequest {
	return entity.RunRequest{Language: "c", Version: FixtureVersion, Source: "opaque source", RequestID: "req-1"}
}

func TestRequestAllowlistAndByteLimits(t *testing.T) {
	for _, language := range []string{"c", "python"} {
		req := validRequest()
		req.Language, req.Source, req.Stdin = language, strings.Repeat("s", MaxSourceBytes), strings.Repeat("i", MaxStdinBytes)
		if err := ValidateRequest(req, NewFixture().Languages()); err != nil {
			t.Fatalf("boundary %s: %v", language, err)
		}
	}
	for name, mutate := range map[string]func(*entity.RunRequest){
		"source too large":     func(r *entity.RunRequest) { r.Source = strings.Repeat("s", MaxSourceBytes+1) },
		"utf8 byte limit":      func(r *entity.RunRequest) { r.Source = strings.Repeat("é", MaxSourceBytes/2+1) },
		"stdin too large":      func(r *entity.RunRequest) { r.Stdin = strings.Repeat("i", MaxStdinBytes+1) },
		"empty source":         func(r *entity.RunRequest) { r.Source = "" },
		"invalid utf8":         func(r *entity.RunRequest) { r.Source = "\xff" },
		"invalid stdin":        func(r *entity.RunRequest) { r.Stdin = "\xff" },
		"unknown language":     func(r *entity.RunRequest) { r.Language = "sh" },
		"alias language":       func(r *entity.RunRequest) { r.Language = "Python" },
		"unpinned version":     func(r *entity.RunRequest) { r.Version = "latest" },
		"real version":         func(r *entity.RunRequest) { r.Version = "3.12" },
		"missing version":      func(r *entity.RunRequest) { r.Version = "" },
		"request id injection": func(r *entity.RunRequest) { r.RequestID = "req\r\nsecret" },
		"request id too large": func(r *entity.RunRequest) { r.RequestID = strings.Repeat("r", 129) },
	} {
		t.Run(name, func(t *testing.T) {
			req := validRequest()
			mutate(&req)
			result, err := NewFixture().Run(context.Background(), req)
			if err == nil || result.Status != StatusProviderError || result.Stdout != "" || !result.Simulated {
				t.Fatalf("invalid request accepted: result=%+v err=%v", result, err)
			}
		})
	}
	if !errors.Is(ValidateRequest(validRequest(), nil), ErrUnsupportedLanguage) {
		t.Fatal("empty allowlist allowed execution")
	}
}

func TestFixtureScenariosAndProvenance(t *testing.T) {
	for _, tc := range []struct {
		stdin, status string
		providerError bool
	}{
		{"", StatusSuccess, false},
		{FixtureCompileError, StatusCompileError, false},
		{FixtureRuntimeError, StatusRuntimeError, false},
		{FixtureTimeout, StatusTimeout, false},
		{FixtureOutage, StatusProviderError, true},
	} {
		t.Run(tc.status, func(t *testing.T) {
			req := validRequest()
			req.Stdin = tc.stdin
			result, err := NewFixture().Run(context.Background(), req)
			if (err != nil) != tc.providerError || result.Status != tc.status || !result.Simulated || result.RunnerVersion != FixtureVersion {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if !strings.Contains(result.Stdout+result.Stderr, "SIMULATED fixture") {
				t.Fatal("simulation not visible")
			}
			if result.RequestID != req.RequestID || len(result.SourceHash) != 64 || result.DurationMS != 0 {
				t.Fatal("incorrect provenance")
			}
			if tc.status == StatusCompileError || tc.status == StatusTimeout || tc.status == StatusProviderError {
				if result.ExitCode != nil {
					t.Fatal("invented process exit")
				}
			}
		})
	}
}

func TestFixtureSourceIsOpaqueData(t *testing.T) {
	// These strings are never evaluated. This is not proof about any real sandbox.
	for _, source := range []string{
		"$(touch /tmp/never-create); rm -rf /; `whoami`",
		"import os; os.system('id')\nopen('/etc/passwd').read()",
		"import socket; socket.create_connection(('127.0.0.1', 80))",
		"#include <stdlib.h>\nint main(){system(\"id\");}",
		"<script>alert('not rendered')</script>",
		FixtureCompileError,
	} {
		req := validRequest()
		req.Source = source
		result, err := NewFixture().Run(context.Background(), req)
		if err != nil || result.Status != StatusSuccess || strings.Contains(result.Stdout+result.Stderr, source) {
			t.Fatalf("source affected execution: %+v %v", result, err)
		}
		hash := sha256.Sum256([]byte(source))
		if result.SourceHash != hex.EncodeToString(hash[:]) {
			t.Fatal("wrong source hash")
		}
	}
}

func TestUnconfiguredAndCancelled(t *testing.T) {
	var disabled Runner = Unconfigured{}
	if disabled.Available() || disabled.Simulated() || len(disabled.Languages()) != 0 {
		t.Fatal("unsafe default")
	}
	result, err := disabled.Run(context.Background(), validRequest())
	if !errors.Is(err, ErrProviderNotConfigured) || result.Status != StatusProviderError || result.ExitCode != nil {
		t.Fatal("disabled runner did work")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, r := range []Runner{disabled, NewFixture()} {
		result, err := r.Run(ctx, validRequest())
		if !errors.Is(err, context.Canceled) || result.Status != StatusProviderError {
			t.Fatal("ignored cancellation")
		}
	}
}

func TestNormalizeBoundsSanitizationAndMetadata(t *testing.T) {
	exit := 0
	for _, text := range []string{strings.Repeat("x", 1<<20), strings.Repeat("é", MaxOutputBytes), strings.Repeat("<", MaxOutputBytes), "safe\x1b\x00‮\r\n\tend\xff"} {
		result, err := NormalizeResult(validRequest(), entity.RunResult{
			Status: StatusSuccess, ExitCode: &exit, RunnerVersion: FixtureVersion, Simulated: true,
			Stdout: text, Stderr: text, RequestID: "forged", SourceHash: "forged",
		})
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result)
		if err != nil || len(encoded) > MaxResultBytes || len(result.Stdout) > MaxOutputBytes || len(result.Stderr) > MaxOutputBytes || !result.Truncated {
			t.Fatal("unbounded result")
		}
		if !utf8.ValidString(result.Stdout) || strings.ContainsAny(result.Stdout, "\x1b\x00‮\r") {
			t.Fatal("unsafe control text")
		}
		if result.RequestID != "req-1" || result.SourceHash == "forged" || !result.Simulated {
			t.Fatal("lost authoritative metadata")
		}
	}
	text := strings.Repeat("é", MaxOutputBytes/2)
	bounded, changed := boundedText(text, MaxOutputBytes)
	if changed || text != bounded {
		t.Fatal("changed output at exact limit")
	}
}

func TestMalformedResultRejected(t *testing.T) {
	for _, result := range []entity.RunResult{
		{Status: "accepted", RunnerVersion: FixtureVersion},
		{Status: StatusSuccess, RunnerVersion: FixtureVersion},
		{Status: StatusRuntimeError, RunnerVersion: "evil\nversion"},
		{Status: StatusTimeout, RunnerVersion: FixtureVersion, DurationMS: -1},
		{Status: StatusTimeout, RunnerVersion: FixtureVersion, DurationMS: 30_001},
	} {
		out, err := NormalizeResult(validRequest(), result)
		if !errors.Is(err, ErrInvalidResult) || out.Status != StatusProviderError {
			t.Fatal("malformed result accepted")
		}
	}
}

func TestFixtureConcurrentAndIndependentCapabilities(t *testing.T) {
	fixture := NewFixture()
	languages := fixture.Languages()
	languages[0].Version = "changed"
	if fixture.Languages()[0].Version != FixtureVersion {
		t.Fatal("mutable shared allowlist")
	}
	want, _ := fixture.Run(context.Background(), validRequest())
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			got, err := fixture.Run(context.Background(), validRequest())
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Error("non-deterministic fixture")
			}
		})
	}
	wg.Wait()
}
