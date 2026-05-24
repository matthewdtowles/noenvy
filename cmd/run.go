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
	Long: `Walks up from the current directory to find the project root, locates the
encrypted file for that project (either ~/.noenvy/projects/<id> or a local
.noenvy if --project was used at init time), decrypts it in memory using the
key from the OS keyring, and exec's the given command with those secrets in
its environment.

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
	// DisableFlagParsing hands us every arg verbatim, including a leading "--"
	// if the user wrote one. Strip it for ergonomics.
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

	root, err := project.FindRoot(cwd)
	if err != nil {
		return err
	}
	projectID, err := project.ID(root)
	if err != nil {
		return err
	}

	path, err := store.Locate(root, projectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("no noenvy data found for project at %s — run `noenvy init` here", root)
		}
		return err
	}

	key, err := keystore.Load(projectID)
	if err != nil {
		if errors.Is(err, keystore.ErrNotFound) {
			return fmt.Errorf("encrypted file %s exists but no key is stored in the OS keyring for it — re-run `noenvy init --force`", path)
		}
		return err
	}

	blob, err := store.Read(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	plaintext, err := cryptobox.Decrypt(key, blob)
	if err != nil {
		return fmt.Errorf("decrypt %s: %w", path, err)
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
