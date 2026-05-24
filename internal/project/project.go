// Package project derives stable identifiers for a noenvy-managed project.
package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
)

// ID returns a stable 32-character hex identifier derived from the absolute
// path to a project's .noenvy file. Used as the keyring account name.
func ID(noenvyPath string) (string, error) {
	abs, err := filepath.Abs(noenvyPath)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	sum := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(sum[:])[:32], nil
}
