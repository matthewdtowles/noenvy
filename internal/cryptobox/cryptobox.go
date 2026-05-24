// Package cryptobox handles symmetric encryption of secret payloads.
//
// File format (binary):
//
//	byte 0       version (currently 0x01)
//	bytes 1..12  12-byte AES-GCM nonce
//	bytes 13..N  ciphertext with appended 16-byte auth tag
package cryptobox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

const (
	Version    byte = 0x01
	KeySize         = 32 // AES-256
	nonceSize       = 12 // GCM standard
	headerSize      = 1 + nonceSize
)

// NewKey returns a freshly generated 32-byte key suitable for Encrypt/Decrypt.
func NewKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return key, nil
}

// Encrypt seals plaintext under key. Output is a self-describing blob containing
// the version byte, nonce, and authenticated ciphertext.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("key must be %d bytes, got %d", KeySize, len(key))
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	out := make([]byte, 0, headerSize+len(plaintext)+gcm.Overhead())
	out = append(out, Version)
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, plaintext, nil), nil
}

// Decrypt verifies and unseals a blob produced by Encrypt.
func Decrypt(key, blob []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("key must be %d bytes, got %d", KeySize, len(key))
	}
	if len(blob) < headerSize {
		return nil, errors.New("blob too short")
	}
	if blob[0] != Version {
		return nil, fmt.Errorf("unsupported format version 0x%02x", blob[0])
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := blob[1:headerSize]
	ct := blob[headerSize:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return pt, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return gcm, nil
}
