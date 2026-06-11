// Package vault is the high-level abstraction over a noenvy project's
// encrypted secrets. It composes project root detection, key lookup from
// the OS keyring, file I/O, and crypto into one type that the cmd layer
// can use without threading those packages itself.
package vault

import (
	"errors"
	"fmt"
	"sort"

	"github.com/matthewdtowles/noenvy/internal/cryptobox"
	"github.com/matthewdtowles/noenvy/internal/keystore"
	"github.com/matthewdtowles/noenvy/internal/project"
	"github.com/matthewdtowles/noenvy/internal/store"
)

// Vault is an in-memory view of a project's secrets, opened for reading or
// mutation. Call Save to persist changes back to disk re-encrypted.
type Vault struct {
	root      string
	projectID string
	path      string
	key       []byte
	vars      map[string]string
}

// Open finds the project root for cwd, loads the encryption key from the
// OS keyring, reads the encrypted file, and decrypts it. Returns a Vault
// suitable for reading via Get/Keys or mutation via Set/Delete/Merge
// followed by Save.
func Open(cwd string) (*Vault, error) {
	root, err := project.FindRoot(cwd)
	if err != nil {
		return nil, err
	}
	projectID, err := project.ID(root)
	if err != nil {
		return nil, err
	}
	path, err := store.Locate(root, projectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("no noenvy vault for project at %s — run `noenvy init` here", root)
		}
		return nil, err
	}
	key, err := keystore.Load(projectID)
	if err != nil {
		if errors.Is(err, keystore.ErrNotFound) {
			return nil, fmt.Errorf("encrypted file %s exists but no key is stored in the OS keyring for it — re-run `noenvy init --force`", path)
		}
		return nil, err
	}
	blob, err := store.Read(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	plaintext, err := cryptobox.Decrypt(key, blob)
	if err != nil {
		return nil, fmt.Errorf("decrypt %s: %w", path, err)
	}
	vars, err := store.ParseEnv(plaintext)
	if err != nil {
		return nil, err
	}
	if vars == nil {
		vars = map[string]string{}
	}
	return &Vault{root: root, projectID: projectID, path: path, key: key, vars: vars}, nil
}

// Path returns the on-disk path to the encrypted file (centralized or in-project).
func (v *Vault) Path() string { return v.path }

// ProjectRoot returns the detected project root for this vault.
func (v *Vault) ProjectRoot() string { return v.root }

// ProjectID returns the keyring account name / centralized filename for this vault.
func (v *Vault) ProjectID() string { return v.projectID }

// Keys returns the vault's keys in sorted order.
func (v *Vault) Keys() []string {
	keys := make([]string, 0, len(v.vars))
	for k := range v.vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Get returns the value for k and whether it was present.
func (v *Vault) Get(k string) (string, bool) {
	val, ok := v.vars[k]
	return val, ok
}

// Has reports whether k is present.
func (v *Vault) Has(k string) bool { _, ok := v.vars[k]; return ok }

// Count returns the number of keys.
func (v *Vault) Count() int { return len(v.vars) }

// Set inserts or replaces a key.
func (v *Vault) Set(k, val string) { v.vars[k] = val }

// Delete removes a key. Returns true if it existed.
func (v *Vault) Delete(k string) bool {
	if _, ok := v.vars[k]; !ok {
		return false
	}
	delete(v.vars, k)
	return true
}

// MergeResult records what happened during a Merge.
type MergeResult struct {
	Added    []string
	Replaced []string
	Skipped  []string
}

// MergeStrategy controls how Merge handles keys that already exist.
type MergeStrategy int

const (
	// MergeError refuses to merge if any incoming key already exists.
	MergeError MergeStrategy = iota
	// MergeOverwrite replaces existing values with incoming ones.
	MergeOverwrite
	// MergeSkipExisting keeps existing values and only adds new keys.
	MergeSkipExisting
)

// ErrMergeConflict is returned by Merge when strategy is MergeError and at
// least one incoming key already exists.
var ErrMergeConflict = errors.New("merge conflict")

// Merge folds incoming keys into the vault per strategy. With MergeError,
// returns ErrMergeConflict (wrapped) listing the conflicting key names.
func (v *Vault) Merge(incoming map[string]string, strategy MergeStrategy) (MergeResult, error) {
	res := MergeResult{}
	// Pre-scan for conflicts in MergeError mode so we can report all of them.
	if strategy == MergeError {
		var conflicts []string
		for k := range incoming {
			if v.Has(k) {
				conflicts = append(conflicts, k)
			}
		}
		if len(conflicts) > 0 {
			sort.Strings(conflicts)
			return res, fmt.Errorf("%w: %v", ErrMergeConflict, conflicts)
		}
	}
	// Apply.
	for k, val := range incoming {
		if v.Has(k) {
			switch strategy {
			case MergeOverwrite:
				v.vars[k] = val
				res.Replaced = append(res.Replaced, k)
			case MergeSkipExisting:
				res.Skipped = append(res.Skipped, k)
			}
		} else {
			v.vars[k] = val
			res.Added = append(res.Added, k)
		}
	}
	sort.Strings(res.Added)
	sort.Strings(res.Replaced)
	sort.Strings(res.Skipped)
	return res, nil
}

// RotateKey generates a new encryption key, writes it to the OS keyring
// (replacing the existing entry), and updates the in-memory key so the
// next Save encrypts with the new one.
func (v *Vault) RotateKey() error {
	newKey, err := cryptobox.NewKey()
	if err != nil {
		return err
	}
	if err := keystore.Store(v.projectID, newKey); err != nil {
		return fmt.Errorf("store new key: %w", err)
	}
	v.key = newKey
	return nil
}

// Save re-serializes the vault to .env-format bytes, encrypts with the
// current key, and writes the encrypted file atomically (temp + rename so
// a crash mid-write can't leave a half-written file).
func (v *Vault) Save() error {
	pairs := make([]store.KV, 0, len(v.vars))
	for _, k := range v.Keys() { // already sorted
		pairs = append(pairs, store.KV{Key: k, Value: v.vars[k]})
	}
	plaintext := store.FormatEnv(pairs)
	blob, err := cryptobox.Encrypt(v.key, plaintext)
	if err != nil {
		return err
	}
	return store.Write(v.path, blob)
}
