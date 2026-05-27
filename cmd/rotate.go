package cmd

import (
	"fmt"
	"os"

	"github.com/matthewdtowles/noenvy/internal/vault"
	"github.com/spf13/cobra"
)

var rotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Generate a new encryption key and re-encrypt the vault with it",
	Long: `Rotates this project's encryption key.

Generates a fresh 32-byte AES-256 key, replaces the entry in your OS
keyring, re-encrypts the vault contents with the new key, and writes the
file atomically. Use after a suspected key compromise, or as routine
hygiene.`,
	RunE: runRotate,
}

func init() {
	rootCmd.AddCommand(rotateCmd)
}

func runRotate(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	v, err := vault.Open(cwd)
	if err != nil {
		return err
	}
	if err := v.RotateKey(); err != nil {
		return err
	}
	if err := v.Save(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Rotated encryption key for project at %s.\n", v.ProjectRoot())
	return nil
}
