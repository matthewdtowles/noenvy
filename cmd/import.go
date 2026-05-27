package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/matthewdtowles/noenvy/internal/store"
	"github.com/matthewdtowles/noenvy/internal/vault"
	"github.com/spf13/cobra"
)

var (
	importOverwrite    bool
	importSkipExisting bool
	importRemoveSource bool
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Merge keys from a .env-format file into the existing vault",
	Long: `Reads <file> in .env format and merges its keys into this project's vault.

By default, errors on the first conflict so you have to pick a strategy:

  --overwrite       replace any conflicting keys with the new values
  --skip-existing   keep current values, only add new keys

You can also pass --remove-source to delete the source file after a
successful import (use this when the file is a temporary copy of secrets
you don't want sitting around in plaintext).`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

func init() {
	importCmd.Flags().BoolVar(&importOverwrite, "overwrite", false, "replace any conflicting keys")
	importCmd.Flags().BoolVar(&importSkipExisting, "skip-existing", false, "keep current values; only add new keys")
	importCmd.Flags().BoolVar(&importRemoveSource, "remove-source", false, "delete the source file after a successful import")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	if importOverwrite && importSkipExisting {
		return errors.New("--overwrite and --skip-existing are mutually exclusive")
	}

	srcPath := args[0]
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", srcPath, err)
	}
	incoming, err := store.ParseEnv(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", srcPath, err)
	}
	if len(incoming) == 0 {
		return fmt.Errorf("%s contains no keys to import", srcPath)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	v, err := vault.Open(cwd)
	if err != nil {
		return err
	}

	strategy := vault.MergeError
	switch {
	case importOverwrite:
		strategy = vault.MergeOverwrite
	case importSkipExisting:
		strategy = vault.MergeSkipExisting
	}

	res, err := v.Merge(incoming, strategy)
	if err != nil {
		if errors.Is(err, vault.ErrMergeConflict) {
			// Extract the conflicting key list from the error message for clarity.
			return fmt.Errorf("%w\nPass --overwrite to replace, or --skip-existing to keep current values.", err)
		}
		return err
	}

	if err := v.Save(); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(res.Added) > 0 {
		fmt.Fprintf(out, "Added %d key(s): %s\n", len(res.Added), strings.Join(res.Added, ", "))
	}
	if len(res.Replaced) > 0 {
		fmt.Fprintf(out, "Replaced %d key(s): %s\n", len(res.Replaced), strings.Join(res.Replaced, ", "))
	}
	if len(res.Skipped) > 0 {
		fmt.Fprintf(out, "Skipped %d key(s) (already present): %s\n", len(res.Skipped), strings.Join(res.Skipped, ", "))
	}
	if len(res.Added) == 0 && len(res.Replaced) == 0 && len(res.Skipped) == 0 {
		fmt.Fprintln(out, "Nothing to do.")
	}

	if importRemoveSource {
		if err := os.Remove(srcPath); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not remove source file %s: %v\n", srcPath, err)
		} else {
			fmt.Fprintf(out, "Removed %s.\n", srcPath)
		}
	}
	return nil
}
