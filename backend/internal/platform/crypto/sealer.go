package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

var ErrInvalidKey = errors.New("session encryption key must be 32 bytes")

type Sealer struct {
	aead cipher.AEAD
}

func NewSealer(rawKey []byte) (Sealer, error) {
	if len(rawKey) != 32 {
		return Sealer{}, ErrInvalidKey
	}
	block, err := aes.NewCipher(rawKey)
	if err != nil {
		return Sealer{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Sealer{}, err
	}
	return Sealer{aead: aead}, nil
}

func (s Sealer) Seal(value string) ([]byte, error) {
	if s.aead == nil {
		return nil, ErrInvalidKey
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, []byte(value), nil), nil
}

func (s Sealer) Open(payload []byte) (string, error) {
	if s.aead == nil || len(payload) < s.aead.NonceSize() {
		return "", ErrInvalidKey
	}
	nonce := payload[:s.aead.NonceSize()]
	plaintext, err := s.aead.Open(nil, nonce, payload[s.aead.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func DecodeBase64Key(value string) ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(value)
}
