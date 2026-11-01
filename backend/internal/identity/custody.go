package identity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/google/uuid"
)

type KeyMaterial struct {
	Reference  string
	WrappedKey []byte
	Nonce      []byte
	PublicKey  []byte
}

// EnvelopeCustody generates Ed25519 keys and immediately seals private key
// material with an AES-256 key-encryption key. There is deliberately no
// plaintext, UID-derived, or recovery-blob fallback.
type EnvelopeCustody struct {
	kek []byte
}

func NewEnvelopeCustody(kek []byte) (*EnvelopeCustody, error) {
	if len(kek) != 32 {
		return nil, fmt.Errorf("a 32-byte key-encryption key is required")
	}
	return &EnvelopeCustody{kek: append([]byte(nil), kek...)}, nil
}

func (c *EnvelopeCustody) Generate() (KeyMaterial, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyMaterial{}, fmt.Errorf("generate identity key: %w", err)
	}
	block, err := aes.NewCipher(c.kek)
	if err != nil {
		return KeyMaterial{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return KeyMaterial{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return KeyMaterial{}, err
	}
	seed := privateKey.Seed()
	wrapped := gcm.Seal(nil, nonce, seed, []byte("baharimrv/identity/v1"))
	for index := range seed {
		seed[index] = 0
	}
	return KeyMaterial{
		Reference:  "local-envelope-kms/" + uuid.NewString(),
		WrappedKey: wrapped,
		Nonce:      nonce,
		PublicKey:  append([]byte(nil), publicKey...),
	}, nil
}
