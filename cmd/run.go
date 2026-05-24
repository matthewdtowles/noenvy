package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/matthewdtowles/noenvy/internal/cryptobox"
	"github.com/matthewdtowles/noenvy/internal/keystore"
	"github.com/matthewdtowles/noenvy/internal/project"
	"github.com/matthewdtowles/noenvy/internal/store"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run -- <command> [args...]",
	Short: "Run a command with secrets from .noenvy injected as environment variables",
	Long: `Walks up from the current directory to find a .noenvy file, looks up its
encryption key in the OS keyring, decrypts the secrets in memory, and exec's
the given command with those secrets in its environment.

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
	// DisableFlagParsing means cobra hands us every arg, including a leading "--"
	// if the user wrote one. Strip it so `noenvy run -- npm start` and
	// `noenvy run npm start` behave the same.
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

	noenvyPath, err := store.Find(cwd)
	if err != nil {
		return err
	}

	projectID, err := project.ID(noenvyPath)
	if err != nil {
		return err
	}

	key, err := keystore.Load(projectID)
	if err != nil {
		if errors.Is(err, keystore.ErrNotFound) {
			return fmt.Errorf("no encryption key in keyring for %s — run `noenvy init` here", noenvyPath)
		}
		return err
	}

	blob, err := store.Read(noenvyPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", noenvyPath, err)
	}

	plaintext, err := cryptobox.Decrypt(key, blob)
	if err != nil {
		return fmt.Errorf("decrypt %s: %w", noenvyPath, err)
	}

	vars, err := store.ParseEnv(plaintext)
	if err != nil {
		return err
	}

	child := exec.Command(args[0], args[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = mergeEnv(os.Environ(), vars)

	if err := child.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Child ran and exited non-zero; propagate its code.
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("exec %s: %w", args[0], err)
	}
	return nil
}

// mergeEnv returns base with any keys from overrides set (and overriding).
func mergeEnv(base []string, overrides map[string]string) []string {
	out := make([]string, 0, len(base)+len(overrides))
	seen := map[string]bool{}
	for k := range overrides {
		seen[k] = true
	}
	for _, kv := range base {
		// strip any existing entry the .noenvy will override
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				if seen[kv[:i]] {
					goto skip
				}
				break
			}
		}
		out = append(out, kv)
	skip:
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}
