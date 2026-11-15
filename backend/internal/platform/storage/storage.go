package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrStorageUnavailable reports a missing or misconfigured object store.
var ErrStorageUnavailable = errors.New("object storage is not configured")

// SignedStore issues time-limited upload and read URLs for a private bucket.
// The browser streams large report files straight to the bucket, bypassing the
// serverless request body limit.
type SignedStore interface {
	Enabled() bool
	SignedUploadURL(ctx context.Context, path string, ttl time.Duration) (string, error)
	SignedDownloadURL(ctx context.Context, path string, ttl time.Duration, downloadName string) (string, error)
}

// SupabaseStorage talks to the Supabase Storage REST API with a service key.
type SupabaseStorage struct {
	baseURL    string
	serviceKey string
	bucket     string
	http       *http.Client
}

func NewSupabase(baseURL, serviceKey, bucket string) *SupabaseStorage {
	return &SupabaseStorage{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		serviceKey: strings.TrimSpace(serviceKey),
		bucket:     strings.TrimSpace(bucket),
		http:       &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *SupabaseStorage) Enabled() bool {
	return s.baseURL != "" && s.serviceKey != "" && s.bucket != ""
}

func (s *SupabaseStorage) SignedUploadURL(ctx context.Context, path string, ttl time.Duration) (string, error) {
	if !s.Enabled() {
		return "", ErrStorageUnavailable
	}
	endpoint := fmt.Sprintf("%s/storage/v1/object/upload/sign/%s/%s", s.baseURL, s.bucket, strings.TrimLeft(path, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceKey)
	body, err := s.do(req)
	if err != nil {
		return "", err
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.URL == "" {
		return "", fmt.Errorf("%w: invalid signed upload response", ErrStorageUnavailable)
	}
	if strings.HasPrefix(out.URL, "http") {
		return out.URL, nil
	}
	return s.baseURL + "/storage/v1/" + strings.TrimPrefix(out.URL, "/storage/v1"), nil
}

func (s *SupabaseStorage) SignedDownloadURL(ctx context.Context, path string, ttl time.Duration, downloadName string) (string, error) {
	if !s.Enabled() {
		return "", ErrStorageUnavailable
	}
	endpoint := fmt.Sprintf("%s/storage/v1/object/sign/%s/%s", s.baseURL, s.bucket, strings.TrimLeft(path, "/"))
	payload, _ := json.Marshal(map[string]int{"expiresIn": int(ttl.Seconds())})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceKey)
	req.Header.Set("Content-Type", "application/json")
	body, err := s.do(req)
	if err != nil {
		return "", err
	}
	var out struct {
		SignedURL string `json:"signedURL"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.SignedURL == "" {
		return "", fmt.Errorf("%w: invalid signed download response", ErrStorageUnavailable)
	}
	full := out.SignedURL
	if !strings.HasPrefix(full, "http") {
		full = s.baseURL + "/storage/v1/" + strings.TrimPrefix(strings.TrimPrefix(full, "/storage/v1"), "/")
	}
	if downloadName != "" {
		sep := "?"
		if strings.Contains(full, "?") {
			sep = "&"
		}
		full += sep + "download=" + url.QueryEscape(downloadName)
	}
	return full, nil
}

func (s *SupabaseStorage) do(req *http.Request) ([]byte, error) {
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStorageUnavailable, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: provider responded %d", ErrStorageUnavailable, resp.StatusCode)
	}
	return body, nil
}

// Unconfigured is the safe default when no private bucket is wired up.
type Unconfigured struct{}

func (Unconfigured) Enabled() bool { return false }
func (Unconfigured) SignedUploadURL(context.Context, string, time.Duration) (string, error) {
	return "", ErrStorageUnavailable
}
func (Unconfigured) SignedDownloadURL(context.Context, string, time.Duration, string) (string, error) {
	return "", ErrStorageUnavailable
}
