package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "noenvy",
	Short: "Encrypt .env files with your OS keyring",
	Long: `noenvy encrypts your .env file into a .noenvy file using a key
stored in your OS keyring. Run any command with secrets injected as
environment variables. No accounts, no servers, no plaintext.`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "noenvy: %v\n", err)
		os.Exit(1)
	}
}
