package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "noenvy",
	Short: "Encrypt .env files with your OS keyring",
	Long: `noenvy encrypts your .env file into an encrypted vault using a key
stored in your OS keyring. Run any command with secrets injected as
environment variables. No accounts, no servers, no plaintext.

Anything that isn't a noenvy subcommand is treated as a command to run
with secrets injected, so these are equivalent:

  noenvy npm start
  noenvy run -- npm start`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	rootCmd.SetArgs(withImplicitRun(os.Args[1:]))
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "noenvy: %v\n", err)
		os.Exit(1)
	}
}

// withImplicitRun rewrites `noenvy <command> [args...]` to
// `noenvy run <command> [args...]` when <command> is not a noenvy
// subcommand, so run acts as the default.
func withImplicitRun(args []string) []string {
	if len(args) == 0 {
		return args
	}
	first := args[0]
	if first == "--" {
		return append([]string{"run"}, args...)
	}
	// Root flags (--help, --version) and cobra's internal completion
	// commands (__complete) pass through untouched.
	if strings.HasPrefix(first, "-") || strings.HasPrefix(first, "__") {
		return args
	}
	// Built-in help and completion commands are normally registered
	// during Execute; register them now so they're seen below.
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()
	for _, c := range rootCmd.Commands() {
		if c.Name() == first || c.HasAlias(first) {
			return args
		}
	}
	return append([]string{"run"}, args...)
}
