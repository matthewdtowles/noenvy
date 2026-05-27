package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/matthewdtowles/noenvy/internal/vault"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run -- <command> [args...]",
	Short: "Run a command with secrets from the vault injected as environment variables",
	Long: `Walks up from the current directory to find the project root, opens the
project's encrypted vault using the key from the OS keyring, decrypts the
secrets in memory, and exec's the given command with those secrets in its
environment.

Use -- to separate noenvy's flags from the command:

  noenvy run -- npm start
  noenvy run -- env | grep API_KEY`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE:               runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return errors.New("no command given (usage: noenvy run -- <command> [args...])")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	v, err := vault.Open(cwd)
	if err != nil {
		return err
	}

	child := exec.Command(args[0], args[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = mergeEnv(os.Environ(), v)

	if err := child.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("exec %s: %w", args[0], err)
	}
	return nil
}

// mergeEnv returns base with vault entries set (and overriding any
// same-named entries from base).
func mergeEnv(base []string, v *vault.Vault) []string {
	keys := v.Keys()
	override := make(map[string]bool, len(keys))
	for _, k := range keys {
		override[k] = true
	}
	out := make([]string, 0, len(base)+len(keys))
	for _, kv := range base {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				if override[kv[:i]] {
					goto skip
				}
				break
			}
		}
		out = append(out, kv)
	skip:
	}
	for _, k := range keys {
		val, _ := v.Get(k)
		out = append(out, k+"="+val)
	}
	return out
}
