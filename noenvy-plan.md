# noenvy — Project Plan

A cross-platform CLI that encrypts your `.env` file with a key stored in your OS keyring, then injects secrets into any command at runtime. No accounts, no servers, no plaintext on disk.

---

## One-line positioning

**Stop putting secrets in .env files.** Works with any language. Plays nicely with Claude Code.

## README top section (build toward this)

> **noenvy** — Stop putting secrets in .env files.
>
> Encrypts your `.env` into a `.noenvy` file using a key stored in your OS keyring. Run any command with secrets injected as environment variables. No accounts, no servers, no plaintext.
>
> ```bash
> noenvy init       # encrypts .env, stores key in keyring
> noenvy npm start  # decrypts in-memory, injects env vars, runs your command
> ```
>
> Works with any language because it runs at the process boundary, not in your code.

---

## Why this exists

Most developers handle local secrets badly:
- Plaintext `.env` files that get accidentally committed
- Cloud secret managers that are overkill for local dev and require accounts/network
- Per-language dotenv libraries that load secrets into the process where any dependency can read them
- Tools like Doppler, Infisical, 1Password CLI that all require an account or sync layer

noenvy is for developers who want their local secrets handled properly with zero infrastructure and zero accounts.

## Threat model (state this clearly in README)

**Protects against:**
- Plaintext secrets sitting on disk
- Accidental git commits of secret files
- Casual filesystem access by other processes

**Does NOT protect against:**
- A compromised user session (attacker with your login has your keyring)
- Malicious code running as your user
- Memory inspection during command execution

This is the same threat model as every other local secret tool. Stating it builds trust with security-minded developers.

---

## v1 scope (ruthlessly narrow)

### Core commands
- `noenvy init` — Read `.env` from current dir, generate encryption key, store in OS keyring, write encrypted file to `~/.noenvy/projects/<project-id>` (default) or to `.noenvy` in the project (with `--project`), add `.env` to `.gitignore` always; add `.noenvy` to `.gitignore` in `--project` mode
- `noenvy run -- <command>` — Decrypt in-memory, inject as env vars, exec the command, never write plaintext to disk. `run` is the default command, so `noenvy <command>` works whenever `<command>` isn't a noenvy subcommand
- `noenvy list` — Show keys currently stored (default keys-only; `--values` shows redacted form only — full values are never printed)
- `noenvy set <KEY>` — Hidden-input prompt (or piped stdin) for value; confirm before overwriting existing key (`--force` to skip)
- `noenvy remove <KEY>` — Delete a key (`--force` to silently succeed if missing). Aliases: `rm`, `unset`
- `noenvy import <file>` — Merge keys from a `.env`-format file. Default errors on conflict; `--overwrite` or `--skip-existing` to choose strategy. `--remove-source` deletes the file after success
- `noenvy rotate` — Generate new encryption key, re-encrypt the vault

### Claude Code skill bootstrapper
- `noenvy install-skill` — Creates a skill in `~/.claude/skills/noenvy/SKILL.md` (global, default) or `./.claude/skills/noenvy/SKILL.md` (project-local). Global install confirms the location once before writing; `--project` writes immediately. `--force` skips all prompts including overwrite confirms.
- Skill teaches Claude Code: when to suggest noenvy, how to wrap commands with `noenvy run --`, how to add secrets safely (interactive set, no echo), how to bulk import, what NOT to do (cat the file, echo values, recommend committing), how to detect setup, and how to interpret common errors.
- Content lives at `internal/skill/SKILL.md` and is embedded into the binary via `go:embed`, so the installed binary doesn't need any companion files.

### What v1 does NOT do (write these down so you don't drift)
- ❌ Sync across machines (different product)
- ❌ Team sharing (that's Vault/Doppler territory)
- ❌ Cloud secret manager integration (v2 maybe)
- ❌ Arbitrary file format support beyond `.env` (v2)
- ❌ Many-to-many matching / abstraction layer (v3, if ever)
- ❌ Interactive interview flows (v2 — happy path + good error messages for v1)
- ❌ Web UI, daemon, anything that runs in the background

---

## Tech stack decision

**Language: Go**

Reasons:
- Single static binary distribution — users download one file, no runtime needed
- Mature keyring library: [`zalando/go-keyring`](https://github.com/zalando/go-keyring) — works on macOS Keychain, Windows Credential Manager, Linux Secret Service
- Strong stdlib crypto (`crypto/aes`, `crypto/rand`)
- Snappy startup time, which matters for a CLI invoked frequently
- Lingua franca of modern CLI tooling (`gh`, `kubectl`, `terraform`) — signals craft

**Key libraries:**
- `github.com/zalando/go-keyring` — OS keyring access
- `github.com/spf13/cobra` — CLI framework (standard choice)
- `github.com/joho/godotenv` — parse `.env` files reliably
- Standard library for crypto: AES-256-GCM is the right choice

**Crypto design:**
- Generate a random 32-byte key on `init`
- Store key in OS keyring under service name `noenvy` and account name = project ID
- Encrypt with AES-256-GCM, store version byte + nonce + ciphertext + auth tag in the encrypted file
- File format: version byte (0x01) + 12-byte nonce + ciphertext + 16-byte auth tag (keep it simple, document it)

---

## Architecture

```
~/.noenvy/projects/<project-id>   # encrypted output (default — centralized)
./project/.env                    # input (gitignored, user-managed)
./project/.noenvy                 # encrypted output (only when --project mode; gitignored)
OS Keyring                        # holds encryption key, never touches disk
```

**Storage default is centralized.** Encrypted files are not designed to be committed in v1 — without the encryption key (which never leaves the local OS keyring), the file is opaque bytes. Team sharing of keys is explicitly out of scope. The `--project` opt-in puts the file inside the project directory for users who prefer keeping per-project files there; it is also gitignored.

**Flow for `noenvy run -- npm start`:**
1. Walk up from cwd looking for `.git`, `package.json`, `Cargo.toml`, `go.mod`, or `pyproject.toml` to find the project root (fall back to cwd)
2. Compute project ID = SHA-256(absolute project root path)[:32]
3. Locate the encrypted file: prefer in-project `<root>/.noenvy`, fall back to `~/.noenvy/projects/<project-id>`
4. Look up encryption key from OS keyring using project ID
5. Decrypt in memory
6. Parse key-value pairs
7. Fork-exec the child command with env vars set (`os/exec` with `Env` field)
8. Stream stdout/stderr through, propagate exit code
9. Never write decrypted values to disk

**Project identification:**
- Project ID derived from the project root's absolute path (SHA-256, hex, first 32 chars)
- Used as both the keyring account name and the filename in centralized storage
- Future: a `.noenvy.toml` config could let users override the project ID for cases where paths change (laptop migration, etc.) — deferred

---

## Status (as of 2026-05-26)

All v1 implementation work is done and pushed to `week-1-init-and-run` (PR #1 open against `main`, 8 commits). Remaining steps are operational: merge, tag, verify the release pipeline produces working artifacts, then launch.

### ✅ Done
- All 8 commands: `init`, `run`, `list`, `set`, `remove`, `import`, `rotate`, `install-skill`
- Centralized storage default (`~/.noenvy/projects/<id>`) with `--project` opt-in for in-project layout
- Vault abstraction (`internal/vault`) with atomic writes (temp + rename)
- Cross-platform release infrastructure: GoReleaser (Mac/Linux/Windows binaries, .deb/.rpm/.apk via nfpm, Homebrew Cask in `matthewdtowles/homebrew-tap`)
- CI workflow: `go vet` + `go test` on ubuntu-latest + macos-latest
- Claude Code skill bundled into the binary via `go:embed`
- README (under-100-word top section, honest threat model, install paths for Mac/Linux/Windows/source, platform-support matrix calling out the Linux keyring caveat, all 8 commands documented, comparison-to-alternatives)
- LICENSE (MIT)
- `matthewdtowles/homebrew-tap` repo created; `HOMEBREW_TAP_GITHUB_TOKEN` secret added to `noenvy`
- Naming check: npm + Homebrew available, GitHub user `noenvy` taken (squatted, dormant) but acceptable papercut, no dominant software trademark

### 🚧 In progress
- PR #1 review and merge to `main`

### Remaining for launch
- Merge PR #1 to `main`
- Tag `v0.0.1` as low-stakes pipeline shakedown (validates Homebrew Cask generation + nfpm packages on real release)
- Verify `brew install matthewdtowles/tap/noenvy` and `sudo dpkg -i noenvy_*.deb` work from the actual release artifacts
- Add demo GIF or asciinema cast to README
- Tag `v0.1.0` as the real launch version
- Launch post(s) per the checklist below

---

## Milestones (historical, for reference)

### Week 1: Foundation
- [x] Repo scaffold with cobra
- [x] `noenvy init` — reads `.env`, generates key, stores in keyring, writes encrypted file
- [x] `noenvy run` — happy path: decrypt, inject, exec
- [x] Cross-platform smoke tests (macOS confirmed; Linux/Windows pending real release)

### Week 2: Round out v1 commands
- [x] `set`, `remove`, `list`, `rotate` (and bonus `import`)
- [x] `.gitignore` handling on init
- [x] Walk-up project root detection with marker files
- [x] Good error messages (no vault, keyring missing, decryption failed, merge conflict, etc.)

### Week 3: Claude Code skill + polish
- [x] `noenvy install-skill` with global/project + confirmation prompts
- [x] Skill content (~130 lines) embedded into binary
- [x] README with top section + threat model + comparison
- [x] LICENSE (MIT), CI for tests + releases
- [ ] Demo GIF or asciinema cast in README (deferred to pre-launch polish)
- [ ] CONTRIBUTING.md (minimal stub in README is sufficient for v1)

### Week 4: Ship
- [x] GoReleaser config for cross-platform binaries (macOS arm64/amd64, Linux amd64/arm64, Windows amd64)
- [x] Homebrew tap setup (repo + PAT + secret wired up)
- [ ] Cut `v0.0.1` to verify the pipeline end-to-end
- [ ] Cut `v0.1.0` as the launchable release
- [ ] Launch post: Hacker News (Tue/Wed morning Eastern), r/programming, r/golang, LinkedIn
- [ ] Reach out directly to 5 people who'd find it useful

---

## Launch checklist

### Before posting
- [x] README top section sings (under 100 words, demo visible immediately) — 56 words, verified
- [ ] Installation works on a fresh machine (test on a VM or borrowed laptop) — pending `v0.0.1` release
- [ ] At least one short demo (GIF or asciinema) in the README
- [x] Threat model section is honest and clear
- [x] Comparison to alternatives included (prose form; could become a table later)
- [x] License chosen (MIT)
- [x] CI green, tests pass, `go vet` clean (will be verified on PR #1's CI run)

### Launch day
- [ ] HN post title: descriptive, no "Show HN: I built..." preamble. Something like "Noenvy – Encrypt .env files with your OS keyring"
- [ ] Be available to respond to comments for the first 2-3 hours
- [ ] Don't argue with critics, address technical questions, fix what's broken
- [ ] LinkedIn post: short, with the demo

### After launch
- [ ] Triage issues thoughtfully — don't promise features, ask why
- [ ] Resist scope creep — point back to ROADMAP for v2 stuff
- [ ] Note who shows up and what they're using it for — that's your real audience signal

---

## Naming check (done — name is "noenvy")

- [x] `noenvy` npm package name available
- [x] `noenvy` available on Homebrew (no core formula by that name)
- [x] GitHub `matthewdtowles/noenvy` repo created; the bare `noenvy` user/org is squatted by a dormant account, but acceptable as a papercut (people will reach the project via the personal-account URL or via Homebrew install)
- [ ] Domain `noenvy.dev` or `noenvy.sh` — not pursued; not needed for v1
- [x] No federally registered software trademark conflicts (scattered non-tech uses in apparel / content creation; low risk for a CLI tool)

Backup names if taken: `envseal`, `envlock`, `keychain-env`, `quietenv`.

---

## Things to deliberately defer (write them down, then forget)

These are good ideas. They will tempt you to expand v1. Resist.

- Arbitrary file format support (any key-value file, not just `.env`)
- Multi-environment support (`.env.staging`, `.env.prod` with profile switching)
- Cloud secret manager backends (AWS Secrets Manager, GCP, Vault as alternative key stores)
- Team sharing via encrypted shared keys
- Interactive interview flows when info is missing
- Many-to-many abstraction layer for sources and destinations
- TUI for managing secrets
- Editor integration (VS Code extension)
- Pre-commit hook to detect leaked secrets
- **Passphrase-based fallback for environments without an OS keyring** (WSL2, headless Linux, Docker containers, devcontainers / Codespaces). Workaround for v1: document that those environments aren't supported and point WSL2 users at installing gnome-keyring. Revisit if real users hit it.
- Multiple-file selection at run time (e.g. `noenvy run --env staging -- ...` to switch between multiple stored vaults per project) — implied by the multi-environment deferral but worth naming separately.
- `--project <path>` flag on mutating commands (set / remove / list / import / rotate) to operate on a project other than cwd. Currently `cd` first.

Put these in `ROADMAP.md` after v1 ships and let user demand prioritize them.

---

## Success criteria

**Minimum win:** Repo exists, works, README is clean. You can point to it in any consulting or job conversation as evidence of your taste and judgment. This is achieved on day one of launch.

**Real win:** 200-500 GitHub stars within 3 months, a handful of issues from real users, one or two contributors. Sufficient signal to legitimately put in your LinkedIn headline.

**Big win:** Steady organic growth, picked up by a newsletter or two, becomes the default suggestion when someone asks "how should I handle local secrets." This compounds over years.

The minimum win is what matters. Everything above it is upside.
