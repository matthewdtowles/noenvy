// Package keystore wraps the OS keyring (macOS Keychain, Windows Credential
// Manager, Linux Secret Service) and stores per-project encryption keys.
package keystore

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const Service = "noenvy"

// ErrNotFound is returned when no key is stored for the given project.
var ErrNotFound = errors.New("no key in keyring for this project")

// Store places key under (Service, projectID) in the OS keyring.
func Store(projectID string, key []byte) error {
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := keyring.Set(Service, projectID, encoded); err != nil {
		return fmt.Errorf("write keyring: %w", err)
	}
	return nil
}

// Load fetches the key for projectID from the OS keyring.
func Load(projectID string) ([]byte, error) {
	encoded, err := keyring.Get(Service, projectID)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read keyring: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode key from keyring: %w", err)
	}
	return key, nil
}

// Delete removes the key for projectID from the OS keyring.
func Delete(projectID string) error {
	if err := keyring.Delete(Service, projectID); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("delete keyring entry: %w", err)
	}
	return nil
}
