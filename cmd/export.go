package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matthewdtowles/noenvy/internal/store"
	"github.com/matthewdtowles/noenvy/internal/vault"
	"github.com/spf13/cobra"
)

var exportForce bool

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Decrypt the vault and write it to a plaintext .env file",
	Long: `Decrypts this project's vault and writes every key to .env in the current
directory — the inverse of init. Use this to migrate secrets out of noenvy,
or to reconstruct a .env you deleted after init.

The output is plaintext, so this asks for confirmation before writing (and
warns when it would overwrite an existing .env). Pass --force to skip the
prompt in scripts. The file is written with 0600 permissions and .env is
kept in the project's .gitignore.`,
	Args: cobra.NoArgs,
	RunE: runExport,
}

func init() {
	exportCmd.Flags().BoolVarP(&exportForce, "force", "f", false, "write without confirmation")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	v, err := vault.Open(cwd)
	if err != nil {
		return err
	}
	if v.Count() == 0 {
		return errors.New("vault is empty — nothing to export")
	}

	outPath := filepath.Join(cwd, ".env")
	if !exportForce {
		verb := "write"
		if _, err := os.Stat(outPath); err == nil {
			verb = "overwrite"
		}
		prompt := fmt.Sprintf("This will %s %s with %d secret(s) in plaintext. Continue? [y/N]: ",
			verb, displayPath(outPath, cwd), v.Count())
		ok, err := confirm(cmd, prompt, false)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(cmd.ErrOrStderr(), "aborted.")
			return nil
		}
	}

	pairs := make([]store.KV, 0, v.Count())
	for _, k := range v.Keys() { // already sorted
		val, _ := v.Get(k)
		pairs = append(pairs, store.KV{Key: k, Value: val})
	}
	if err := store.Write(outPath, store.FormatEnv(pairs)); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	out := cmd.OutOrStdout()
	added, err := ensureGitignored(v.ProjectRoot(), []string{filepath.Base(outPath)})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not update .gitignore in %s: %v\n", v.ProjectRoot(), err)
	}
	if len(added) > 0 {
		fmt.Fprintf(out, "Added to %s/.gitignore: %s\n", displayPath(v.ProjectRoot(), cwd), strings.Join(added, ", "))
	}
	fmt.Fprintf(out, "Wrote %d key(s) to %s (plaintext).\n", v.Count(), displayPath(outPath, cwd))
	return nil
}
