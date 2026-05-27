// Package skill bundles the Claude Code skill content for noenvy at
// compile time, so the installed binary doesn't need any companion files.
package skill

import _ "embed"

// Template is the SKILL.md content shipped with the binary.
//
//go:embed SKILL.md
var Template string

// FileName is the standard filename for a Claude Code skill.
const FileName = "SKILL.md"

// DirName is the per-skill directory name inside .claude/skills/.
const DirName = "noenvy"
