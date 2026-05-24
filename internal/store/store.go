// Package store handles reading and writing the on-disk .noenvy file and
// finding it by walking up from the current working directory.
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const Filename = ".noenvy"

// ErrNotFound indicates no .noenvy file was found in this dir or any parent.
var ErrNotFound = errors.New(".noenvy file not found in current or any parent directory")

// Find walks up from startDir looking for a .noenvy file. Returns its absolute path.
func Find(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, Filename)
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("stat %s: %w", candidate, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}

// Read returns the raw bytes of a .noenvy file.
func Read(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// Write writes data to path with restrictive (0600) permissions.
func Write(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

// ParseEnv parses a decrypted .env-format payload into a key/value map.
func ParseEnv(plaintext []byte) (map[string]string, error) {
	m, err := godotenv.Unmarshal(string(plaintext))
	if err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	return m, nil
}

// FormatEnv serializes a key/value map back to .env file syntax. Keys are
// emitted in the order provided.
func FormatEnv(pairs []KV) []byte {
	var b strings.Builder
	for _, kv := range pairs {
		b.WriteString(kv.Key)
		b.WriteByte('=')
		b.WriteString(quote(kv.Value))
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// KV is an ordered key/value pair for env serialization.
type KV struct {
	Key   string
	Value string
}

func quote(v string) string {
	if strings.ContainsAny(v, " \t\n\"'#=") {
		return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
	}
	return v
}
