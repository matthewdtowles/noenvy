package cmd

import (
	"fmt"
	"os"

	"github.com/matthewdtowles/noenvy/internal/vault"
	"github.com/spf13/cobra"
)

var listShowValues bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the keys in this project's vault",
	Long: `Prints the keys stored in this project's vault, one per line, sorted.

With --values, also prints a redacted form of each value (first three and
last three characters, with *** in the middle; values eight characters or
shorter are fully masked). Full values are never printed; if you need to
see the raw value of a key, use ` + "`noenvy run -- env | grep KEY`" + ` and accept
the terminal exposure.`,
	RunE: runList,
}

func init() {
	listCmd.Flags().BoolVarP(&listShowValues, "values", "v", false, "also print a redacted form of each value")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	v, err := vault.Open(cwd)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	for _, k := range v.Keys() {
		if listShowValues {
			val, _ := v.Get(k)
			fmt.Fprintf(out, "%s=%s\n", k, redact(val))
		} else {
			fmt.Fprintln(out, k)
		}
	}
	return nil
}

// redact returns a masked form of v that exposes at most the first three and
// last three characters. Short values (≤ 8 chars) are fully masked so nothing
// is leaked.
func redact(v string) string {
	if len(v) <= 8 {
		return "***"
	}
	return v[:3] + "***" + v[len(v)-3:]
}
