package authplatform

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrProviderNotConfigured = errors.New("identity provider is not configured")
	ErrProviderUnavailable   = errors.New("identity provider is unavailable")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrInvalidAuthCode       = errors.New("invalid authorization code")
)

type Identity struct {
	AuthUserID string
	Email      string
	Name       string
}

type IdentityProvider interface {
	ExchangeCode(context.Context, string, string) (Identity, error)
	PasswordLogin(context.Context, string, string) (Identity, error)
}

type FixtureProvider struct {
	mu         sync.Mutex
	identities map[string]Identity
	used       map[string]bool
	expires    time.Time
}

func NewFixtureProvider() *FixtureProvider {
	identities := map[string]Identity{}
	for _, item := range []struct{ id, email, name string }{
		{"student", "mahasiswa", "Mahasiswa Fixture"},
		{"assistant", "asisten", "Asisten Tanpa Divisi"},
		{"secretary", "sekretaris", "Asisten Sekretaris"},
		{"treasurer", "bendahara", "Asisten Bendahara"},
		{"k3", "k3", "Asisten K3"},
		{"labops", "labops", "Asisten Labops"},
		{"education", "pendidikan", "Asisten Pendidikan"},
		{"pdd", "pdd", "Asisten PDD"},
		{"coordinator", "koordinator", "Koordinator Fixture"},
		{"borrower", "peminjam", "Peminjam Fixture"},
		{"scoped", "scoped", "Asisten Scoped"},
	} {
		id := "fixture-" + item.id
		identities[id] = Identity{AuthUserID: id, Email: item.email + "@example.test", Name: item.name}
	}
	return &FixtureProvider{identities: identities, used: map[string]bool{}, expires: time.Now().Add(5 * time.Minute)}
}

func (p *FixtureProvider) ExchangeCode(ctx context.Context, code, verifier string) (Identity, error) {
	if err := ctx.Err(); err != nil {
		return Identity{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	identity, ok := p.identities[code]
	if !ok || p.used[code] || verifier == "" || time.Now().After(p.expires) {
		return Identity{}, ErrInvalidAuthCode
	}
	p.used[code] = true
	return identity, nil
}

func (p *FixtureProvider) PasswordLogin(ctx context.Context, email, password string) (Identity, error) {
	if err := ctx.Err(); err != nil {
		return Identity{}, err
	}
	if password != "fixture-password" {
		return Identity{}, ErrInvalidCredentials
	}
	for _, identity := range p.identities {
		if identity.Email == email {
			return identity, nil
		}
	}
	return Identity{}, ErrInvalidCredentials
}

type UnconfiguredProvider struct{}

func (UnconfiguredProvider) ExchangeCode(context.Context, string, string) (Identity, error) {
	return Identity{}, ErrProviderNotConfigured
}
func (UnconfiguredProvider) PasswordLogin(context.Context, string, string) (Identity, error) {
	return Identity{}, ErrProviderNotConfigured
}
