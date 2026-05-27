package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/matthewdtowles/noenvy/internal/skill"
	"github.com/spf13/cobra"
)

var (
	installSkillProjectLocal bool
	installSkillForce        bool
)

var installSkillCmd = &cobra.Command{
	Use:   "install-skill",
	Short: "Install a Claude Code skill that teaches Claude how to use noenvy",
	Long: `Writes a Claude Code skill file that tells Claude when to suggest noenvy,
how to wrap commands with it, how to add secrets safely, and what NOT to do.

Default location is global, so the skill applies to any project:

  ~/.claude/skills/noenvy/SKILL.md

Pass --project to install the skill into the current project only:

  <cwd>/.claude/skills/noenvy/SKILL.md

The global install confirms the location before writing. The project-local
install writes immediately (since you explicitly chose --project). In both
cases, an existing SKILL.md is confirmed before overwriting. Pass --force
to skip all prompts (useful for scripts and CI).`,
	RunE: runInstallSkill,
}

func init() {
	installSkillCmd.Flags().BoolVar(&installSkillProjectLocal, "project", false, "install in <cwd>/.claude/skills/ instead of the global ~/.claude/skills/")
	installSkillCmd.Flags().BoolVarP(&installSkillForce, "force", "f", false, "overwrite without prompting and skip the location-confirmation prompt")
	rootCmd.AddCommand(installSkillCmd)
}

func runInstallSkill(cmd *cobra.Command, args []string) error {
	baseDir, err := skillBaseDir(installSkillProjectLocal)
	if err != nil {
		return err
	}
	skillPath := filepath.Join(baseDir, skill.FileName)

	// For the global default, confirm the location once (the user might not
	// realize the skill applies to all projects). For --project, the user
	// was explicit, no need to ask.
	if !installSkillProjectLocal && !installSkillForce {
		ok, err := confirm(cmd, fmt.Sprintf("Install noenvy skill at %s (global — applies to all projects)? [Y/n]: ", skillPath), true)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(cmd.ErrOrStderr(), "aborted. (Pass --project to install in this project only, or --force to skip this prompt.)")
			return nil
		}
	}

	if _, err := os.Stat(skillPath); err == nil && !installSkillForce {
		ok, err := confirm(cmd, fmt.Sprintf("%s already exists. Overwrite? [y/N]: ", skillPath), false)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(cmd.ErrOrStderr(), "aborted.")
			return nil
		}
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", skillPath, err)
	}

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", baseDir, err)
	}
	if err := os.WriteFile(skillPath, []byte(skill.Template), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", skillPath, err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Installed noenvy skill at %s\n", skillPath)
	if installSkillProjectLocal {
		fmt.Fprintln(out, "Claude Code will use it in sessions opened in this project.")
	} else {
		fmt.Fprintln(out, "Claude Code will use it in any project.")
	}
	return nil
}

func skillBaseDir(projectLocal bool) (string, error) {
	if projectLocal {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".claude", "skills", skill.DirName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "skills", skill.DirName), nil
}
