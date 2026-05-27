package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/matthewdtowles/noenvy/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var setForce bool

var setCmd = &cobra.Command{
	Use:   "set <KEY>",
	Short: "Add or replace a single secret in the vault",
	Long: `Adds KEY to the vault, or replaces its value if it already exists.

If stdin is a terminal, prompts for the value with input hidden. If stdin
is piped or redirected, reads the value from stdin (with the trailing
newline stripped). This means:

  noenvy set DATABASE_URL                # interactive, hidden input
  echo 'postgres://...' | noenvy set DB  # script-friendly

If KEY already exists, asks for confirmation before overwriting. Pass
--force to skip the prompt (useful for scripts).`,
	Args: cobra.ExactArgs(1),
	RunE: runSet,
}

func init() {
	setCmd.Flags().BoolVarP(&setForce, "force", "f", false, "overwrite without confirmation")
	rootCmd.AddCommand(setCmd)
}

func runSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	if err := validateKey(key); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	v, err := vault.Open(cwd)
	if err != nil {
		return err
	}

	existed := v.Has(key)
	if existed && !setForce {
		ok, err := confirm(cmd, fmt.Sprintf("%s already exists. Overwrite? [y/N]: ", key))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(cmd.ErrOrStderr(), "aborted.")
			return nil
		}
	}

	value, err := readSecretValue(cmd)
	if err != nil {
		return err
	}

	v.Set(key, value)
	if err := v.Save(); err != nil {
		return err
	}

	if existed {
		fmt.Fprintf(cmd.OutOrStdout(), "Updated %s.\n", key)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Set %s.\n", key)
	}
	return nil
}

// readSecretValue gets the value from either a TTY prompt (hidden input)
// or from piped stdin.
func readSecretValue(cmd *cobra.Command) (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(cmd.ErrOrStderr(), "Value (hidden): ")
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(cmd.ErrOrStderr()) // newline after hidden input
		if err != nil {
			return "", fmt.Errorf("read value: %w", err)
		}
		val := string(raw)
		if val == "" {
			return "", errors.New("empty value")
		}
		return val, nil
	}
	// Non-TTY: read entire stdin, strip the single trailing newline if any.
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	val := strings.TrimRight(string(data), "\n")
	if val == "" {
		return "", errors.New("empty value on stdin")
	}
	return val, nil
}

// confirm prompts the user (via stderr) for a yes/no answer. Returns true
// only on an explicit "y" or "yes" (case-insensitive). Anything else,
// including empty input, is "no".
func confirm(cmd *cobra.Command, prompt string) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// No way to confirm interactively; refuse rather than guess.
		return false, errors.New("refusing to overwrite without --force in non-interactive mode")
	}
	fmt.Fprint(cmd.ErrOrStderr(), prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes", nil
}

// validateKey enforces .env-style key names: non-empty, letters / digits /
// underscores, not starting with a digit. Mirrors what most dotenv parsers
// will accept on the consuming side.
func validateKey(k string) error {
	if k == "" {
		return errors.New("key cannot be empty")
	}
	for i, r := range k {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return fmt.Errorf("invalid character %q in key %q (keys must match [A-Za-z_][A-Za-z0-9_]*)", r, k)
		}
	}
	return nil
}
