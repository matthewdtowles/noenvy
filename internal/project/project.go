// Package project derives stable identifiers and locates the root of a
// noenvy-managed project on disk.
package project

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// RootMarkers is the set of files/dirs that signal a project root when
// walking up from the current working directory. First-match wins, which
// gives correct results in monorepos (innermost project is preferred).
var RootMarkers = []string{
	".git",
	"package.json",
	"Cargo.toml",
	"go.mod",
	"pyproject.toml",
}

// FindRoot walks up from startDir looking for a directory that contains any
// entry in RootMarkers. Returns that directory's absolute path. If no marker
// is found by the time the walk reaches the filesystem root, the absolute
// form of startDir itself is returned — so callers always get a usable root.
func FindRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	start := dir
	for {
		for _, marker := range RootMarkers {
			_, err := os.Stat(filepath.Join(dir, marker))
			if err == nil {
				return dir, nil
			}
			if !errors.Is(err, fs.ErrNotExist) {
				return "", fmt.Errorf("stat %s: %w", filepath.Join(dir, marker), err)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start, nil
		}
		dir = parent
	}
}

// ID returns a stable 32-character hex identifier derived from the absolute
// path to a project's root directory. Used as the keyring account name and
// as the filename in centralized storage.
func ID(rootDir string) (string, error) {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	sum := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(sum[:])[:32], nil
}
