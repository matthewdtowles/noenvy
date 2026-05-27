package cmd

import (
	"fmt"
	"os"

	"github.com/matthewdtowles/noenvy/internal/vault"
	"github.com/spf13/cobra"
)

var removeForce bool

var removeCmd = &cobra.Command{
	Use:     "remove <KEY>",
	Aliases: []string{"rm", "unset"},
	Short:   "Delete a key from the vault",
	Long: `Removes KEY from the vault. Errors if the key doesn't exist; pass --force
to silently succeed in that case (useful for idempotent scripts).`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

func init() {
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "do not error if the key doesn't exist")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	key := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	v, err := vault.Open(cwd)
	if err != nil {
		return err
	}

	if !v.Delete(key) {
		if removeForce {
			return nil
		}
		return fmt.Errorf("no such key: %s", key)
	}

	if err := v.Save(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Removed %s.\n", key)
	return nil
}
