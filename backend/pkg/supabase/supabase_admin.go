package supabase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Admin memanggil Supabase Admin API pakai service_role. Hanya backend.
type Admin struct {
	URL        string
	ServiceKey string
	HTTP       *http.Client
}

func NewAdmin(url, serviceKey string) *Admin {
	return &Admin{
		URL:        strings.TrimRight(url, "/"),
		ServiceKey: serviceKey,
		HTTP:       &http.Client{Timeout: 10 * time.Second},
	}
}

type adminUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// CreateAuthUser bikin akun auth via Admin API. emailConfirm=true → langsung aktif (kita pakai true).
func (s *Admin) CreateAuthUser(email, password string, emailConfirm bool) (string, error) {
	if s.URL == "" || s.ServiceKey == "" {
		return "", fmt.Errorf("SUPABASE_URL / SUPABASE_SERVICE_KEY belum diset")
	}
	b, _ := json.Marshal(map[string]interface{}{"email": email, "password": password, "email_confirm": emailConfirm})
	req, _ := http.NewRequest(http.MethodPost, s.URL+"/auth/v1/admin/users", bytes.NewReader(b))
	req.Header.Set("apikey", s.ServiceKey)
	req.Header.Set("Authorization", "Bearer "+s.ServiceKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out adminUser
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode == http.StatusUnprocessableEntity {
		return "", fmt.Errorf("email sudah terdaftar di Supabase Auth")
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("Supabase Admin API %d", resp.StatusCode)
	}
	return out.ID, nil
}

func (s *Admin) DeleteAuthUser(uid string) error {
	req, _ := http.NewRequest(http.MethodDelete, s.URL+"/auth/v1/admin/users/"+uid, nil)
	req.Header.Set("apikey", s.ServiceKey)
	req.Header.Set("Authorization", "Bearer "+s.ServiceKey)
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("hapus user Supabase gagal: %d", resp.StatusCode)
	}
	return nil
}

func (s *Admin) SendRecoveryOTP(email string) error {
	if s.URL == "" || s.ServiceKey == "" {
		return fmt.Errorf("SUPABASE_URL / SERVICE_KEY belum diset")
	}
	b, _ := json.Marshal(map[string]string{"email": email})
	req, _ := http.NewRequest(http.MethodPost, s.URL+"/auth/v1/recover", bytes.NewReader(b))
	req.Header.Set("apikey", s.ServiceKey)
	req.Header.Set("Authorization", "Bearer "+s.ServiceKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Supabase recover %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (s *Admin) VerifyRecoveryOTP(email, token, newPassword string) error {
	if s.URL == "" || s.ServiceKey == "" {
		return fmt.Errorf("SUPABASE_URL / SERVICE_KEY belum diset")
	}
	b, _ := json.Marshal(map[string]string{"type": "recovery", "email": email, "token": token})
	req, _ := http.NewRequest(http.MethodPost, s.URL+"/auth/v1/verify", bytes.NewReader(b))
	req.Header.Set("apikey", s.ServiceKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("otp_invalid")
	}
	var out struct {
		User struct{ ID string `json:"id"` } `json:"user"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.User.ID == "" {
		return fmt.Errorf("otp_invalid")
	}
	return s.updatePassword(out.User.ID, newPassword)
}

func (s *Admin) updatePassword(uid, newPassword string) error {
	b, _ := json.Marshal(map[string]string{"password": newPassword})
	req, _ := http.NewRequest(http.MethodPut, s.URL+"/auth/v1/admin/users/"+uid, bytes.NewReader(b))
	req.Header.Set("apikey", s.ServiceKey)
	req.Header.Set("Authorization", "Bearer "+s.ServiceKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("gagal set password Supabase: %d", resp.StatusCode)
	}
	return nil
}

// UpdateAuthUserEmail mengubah email akun auth Supabase — dipakai saat pengguna
// mengganti email sendiri dari halaman profil (email inilah kanal OTP lupa-password).
func (s *Admin) UpdateAuthUserEmail(uid, email string) error {
	b, _ := json.Marshal(map[string]string{"email": email})
	req, _ := http.NewRequest(http.MethodPut, s.URL+"/auth/v1/admin/users/"+uid, bytes.NewReader(b))
	req.Header.Set("apikey", s.ServiceKey)
	req.Header.Set("Authorization", "Bearer "+s.ServiceKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnprocessableEntity {
		return fmt.Errorf("email sudah terdaftar di Supabase Auth")
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("gagal ubah email Supabase: %d", resp.StatusCode)
	}
	return nil
}
