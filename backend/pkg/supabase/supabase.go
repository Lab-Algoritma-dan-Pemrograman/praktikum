package supabase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client membungkus operasi Supabase Storage (REST API).
// Dipakai untuk upload foto asisten, PDF modul/pedoman, dan gambar flowchart.
type Client struct {
	baseURL    string
	serviceKey string
	bucket     string
	http       *http.Client
}

func New(baseURL, serviceKey, bucket string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		serviceKey: serviceKey,
		bucket:     bucket,
		http:       &http.Client{Timeout: 30 * time.Second},
	}
}

// Enabled menandakan apakah konfigurasi Supabase tersedia.
func (c *Client) Enabled() bool {
	return c.baseURL != "" && c.serviceKey != ""
}

// Upload mengunggah file ke path tertentu dalam bucket dan mengembalikan URL publik.
// upsert=true akan menimpa file yang sudah ada.
func (c *Client) Upload(path string, content []byte, contentType string, upsert bool) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("supabase belum dikonfigurasi")
	}
	path = strings.TrimLeft(path, "/")
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", c.baseURL, c.bucket, path)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	req.Header.Set("Content-Type", contentType)
	if upsert {
		req.Header.Set("x-upsert", "true")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("upload gagal (%d): %s", resp.StatusCode, string(body))
	}
	return c.PublicURL(path), nil
}

// PublicURL membangun URL publik untuk objek pada bucket public.
func (c *Client) PublicURL(path string) string {
	path = strings.TrimLeft(path, "/")
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", c.baseURL, c.bucket, path)
}

// SignedUploadURL membuat URL unggah bertanda tangan (berlaku terbatas waktu).
// Dipakai agar berkas besar (laporan PDF/DOCX) mengalir browser -> Supabase
// langsung, tanpa melewati backend (yang dibatasi ukuran body di serverless).
func (c *Client) SignedUploadURL(path string, ttl time.Duration) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("supabase belum dikonfigurasi")
	}
	path = strings.TrimLeft(path, "/")
	endpoint := fmt.Sprintf("%s/storage/v1/object/upload/sign/%s/%s", c.baseURL, c.bucket, path)

	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("signed upload gagal (%d): %s", resp.StatusCode, string(body))
	}

	var out struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("respons signed upload tidak valid: %w", err)
	}
	// Supabase mengembalikan path relatif (mis. "/object/upload/sign/...").
	if strings.HasPrefix(out.URL, "http") {
		return out.URL, nil
	}
	return c.baseURL + "/storage/v1" + strings.TrimPrefix(out.URL, "/storage/v1"), nil
}

// SignedDownloadURL membuat URL baca bertanda tangan (berlaku terbatas waktu).
// downloadName != "" menambahkan ?download=<nama> agar respons jadi attachment.
// Bucket laporan bersifat privat, jadi semua baca lewat url ini.
func (c *Client) SignedDownloadURL(path string, ttl time.Duration, downloadName string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("supabase belum dikonfigurasi")
	}
	path = strings.TrimLeft(path, "/")
	endpoint := fmt.Sprintf("%s/storage/v1/object/sign/%s/%s", c.baseURL, c.bucket, path)

	payload, _ := json.Marshal(map[string]int{"expiresIn": int(ttl.Seconds())})
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("signed download gagal (%d): %s", resp.StatusCode, string(body))
	}

	var out struct {
		SignedURL string `json:"signedURL"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("respons signed download tidak valid: %w", err)
	}
	full := c.baseURL + "/storage/v1" + strings.TrimPrefix(out.SignedURL, "/storage/v1")
	if !strings.HasPrefix(out.SignedURL, "http") && !strings.HasPrefix(out.SignedURL, "/storage/v1") {
		full = c.baseURL + "/storage/v1" + strings.TrimPrefix(out.SignedURL, "/")
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

// Delete menghapus objek dari bucket.
func (c *Client) Delete(path string) error {
	if !c.Enabled() {
		return fmt.Errorf("supabase belum dikonfigurasi")
	}
	path = strings.TrimLeft(path, "/")
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", c.baseURL, c.bucket, path)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete gagal (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}
