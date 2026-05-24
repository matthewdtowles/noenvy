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
	initEnvFile      string
	initForce        bool
	initProjectLocal bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Encrypt the local .env file and store its key in the OS keyring",
	Long: `Reads .env from the current directory, generates a new encryption key,
stores it in the OS keyring, and writes an encrypted file.

By default the encrypted file lives at ~/.noenvy/projects/<project-id>, keyed
to the project root (detected by walking up for .git, package.json, Cargo.toml,
go.mod, or pyproject.toml). Pass --project to store it as .noenvy inside the
project directory instead.

.env is always added to .gitignore. In --project mode, .noenvy is also added.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initEnvFile, "env-file", ".env", "path to the .env file to encrypt")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite any existing encrypted file and replace the keyring entry")
	initCmd.Flags().BoolVar(&initProjectLocal, "project", false, "store the encrypted file inside the project (.noenvy) instead of under ~/.noenvy")
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

	root, err := project.FindRoot(cwd)
	if err != nil {
		return err
	}
	projectID, err := project.ID(root)
	if err != nil {
		return err
	}

	var outPath string
	if initProjectLocal {
		outPath = store.InProjectPath(root)
	} else {
		outPath, err = store.CentralizedPath(projectID)
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(outPath); err == nil && !initForce {
		return fmt.Errorf("%s already exists — pass --force to overwrite", outPath)
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
	if err := store.Write(outPath, blob); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	ignoreList := []string{filepath.Base(envPath)}
	if initProjectLocal {
		ignoreList = append(ignoreList, store.Filename)
	}
	added, err := ensureGitignored(root, ignoreList)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not update .gitignore in %s: %v\n", root, err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Encrypted %s → %s\n", displayPath(envPath, cwd), displayPath(outPath, cwd))
	fmt.Fprintf(out, "Key stored in OS keyring (service=%s, project=%s)\n", keystore.Service, projectID)
	if len(added) > 0 {
		fmt.Fprintf(out, "Added to %s/.gitignore: %s\n", displayPath(root, cwd), strings.Join(added, ", "))
	}
	fmt.Fprintln(out, "\nNext: noenvy run -- <your command>")
	return nil
}

// displayPath returns a short, human-friendly rendering of a path: relative
// to cwd if it's inside cwd, or as-is otherwise.
func displayPath(path, cwd string) string {
	if rel, err := filepath.Rel(cwd, path); err == nil && !strings.HasPrefix(rel, "..") {
		if rel == "." {
			return path
		}
		return rel
	}
	return path
}

// ensureGitignored appends any of entries that aren't already present in
// <dir>/.gitignore (creating the file if needed). Returns the entries actually
// added.
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
