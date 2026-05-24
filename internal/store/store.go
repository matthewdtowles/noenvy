// Package store handles reading and writing the on-disk .noenvy file in
// either layout (centralized under ~/.noenvy or in-project at <root>/.noenvy)
// and parses decrypted payloads as .env content.
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

const (
	// Filename is the on-disk name when storing in the project directory.
	Filename = ".noenvy"
	// UserDirName is the per-user noenvy directory under $HOME.
	UserDirName = ".noenvy"
	// ProjectsSubdir holds one encrypted file per project under UserDirName.
	ProjectsSubdir = "projects"
)

// ErrNotFound indicates no .noenvy file exists for this project in either layout.
var ErrNotFound = errors.New("no .noenvy found for this project (run `noenvy init`)")

// UserDir returns the absolute path to ~/.noenvy.
func UserDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, UserDirName), nil
}

// CentralizedPath returns the path to the encrypted file for a given project
// ID in the centralized layout: ~/.noenvy/projects/<projectID>.
func CentralizedPath(projectID string) (string, error) {
	dir, err := UserDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ProjectsSubdir, projectID), nil
}

// InProjectPath returns the path to the encrypted file in the project layout:
// <root>/.noenvy.
func InProjectPath(root string) string {
	return filepath.Join(root, Filename)
}

// Locate returns the path to whichever .noenvy file already exists for this
// project, preferring the in-project layout when both happen to be present.
// Returns ErrNotFound if neither layout has a file.
func Locate(root, projectID string) (string, error) {
	inProj := InProjectPath(root)
	if exists(inProj) {
		return inProj, nil
	}
	central, err := CentralizedPath(projectID)
	if err != nil {
		return "", err
	}
	if exists(central) {
		return central, nil
	}
	return "", ErrNotFound
}

// Read returns the raw bytes of an encrypted .noenvy file.
func Read(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// Write writes data to path with restrictive (0600) permissions, creating
// any missing parent directories (0700) along the way.
func Write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", path, err)
	}
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

// KV is an ordered key/value pair for env serialization.
type KV struct {
	Key   string
	Value string
}

// FormatEnv serializes a list of key/value pairs back to .env file syntax.
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

func quote(v string) string {
	if strings.ContainsAny(v, " \t\n\"'#=") {
		return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
	}
	return v
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	// Unknown stat error — treat as non-existent; callers will hit a clearer
	// error if they try to read it.
	return false
}
