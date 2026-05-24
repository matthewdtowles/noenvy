package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/matthewdtowles/noenvy/internal/cryptobox"
	"github.com/matthewdtowles/noenvy/internal/keystore"
	"github.com/matthewdtowles/noenvy/internal/project"
	"github.com/matthewdtowles/noenvy/internal/store"
	"github.com/spf13/cobra"
)

var (
	initEnvFile string
	initForce   bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Encrypt the local .env file and store its key in the OS keyring",
	Long: `Reads .env from the current directory, generates a new encryption key,
stores it in the OS keyring, and writes an encrypted .noenvy file.

Also adds .env and .noenvy to .gitignore so neither is accidentally committed.
If you want to commit the encrypted .noenvy file (so collaborators with the
key can use it), remove that line from .gitignore yourself.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initEnvFile, "env-file", ".env", "path to the .env file to encrypt")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing .noenvy and replace any existing keyring entry")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	envPath := initEnvFile
	if !filepath.IsAbs(envPath) {
		envPath = filepath.Join(cwd, envPath)
	}
	plaintext, err := os.ReadFile(envPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("no %s file in %s — create one first or pass --env-file", initEnvFile, cwd)
		}
		return fmt.Errorf("read %s: %w", envPath, err)
	}

	if _, err := store.ParseEnv(plaintext); err != nil {
		return fmt.Errorf("validate %s: %w", envPath, err)
	}

	noenvyPath := filepath.Join(cwd, store.Filename)
	if _, err := os.Stat(noenvyPath); err == nil && !initForce {
		return fmt.Errorf("%s already exists — pass --force to overwrite", noenvyPath)
	}

	projectID, err := project.ID(noenvyPath)
	if err != nil {
		return err
	}

	key, err := cryptobox.NewKey()
	if err != nil {
		return err
	}

	blob, err := cryptobox.Encrypt(key, plaintext)
	if err != nil {
		return err
	}

	if err := keystore.Store(projectID, key); err != nil {
		return fmt.Errorf("store key in OS keyring: %w", err)
	}

	if err := store.Write(noenvyPath, blob); err != nil {
		return fmt.Errorf("write %s: %w", noenvyPath, err)
	}

	added, err := ensureGitignored(cwd, []string{".env", store.Filename})
	if err != nil {
		// Non-fatal: encryption succeeded, just warn.
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not update .gitignore: %v\n", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Encrypted %s → %s\n", filepath.Base(envPath), store.Filename)
	fmt.Fprintf(cmd.OutOrStdout(), "Key stored in OS keyring (service=%s, project=%s)\n", keystore.Service, projectID)
	if len(added) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Added to .gitignore: %s\n", strings.Join(added, ", "))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\nNext: noenvy run -- <your command>")
	return nil
}

// ensureGitignored appends any of entries that aren't already present in
// .gitignore (creating it if needed). Returns the entries actually added.
func ensureGitignored(dir string, entries []string) ([]string, error) {
	path := filepath.Join(dir, ".gitignore")
	existing := map[string]bool{}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		existing[strings.TrimSpace(line)] = true
	}

	var added []string
	var toAppend []string
	for _, e := range entries {
		if !existing[e] {
			added = append(added, e)
			toAppend = append(toAppend, e)
		}
	}
	if len(toAppend) == 0 {
		return nil, nil
	}

	var buf strings.Builder
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		buf.WriteByte('\n')
	}
	if len(data) > 0 {
		buf.WriteString("\n# added by noenvy\n")
	} else {
		buf.WriteString("# added by noenvy\n")
	}
	for _, e := range toAppend {
		buf.WriteString(e)
		buf.WriteByte('\n')
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.WriteString(buf.String()); err != nil {
		return nil, err
	}
	return added, nil
}
