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
> noenvy init              # encrypts .env, stores key in keyring
> noenvy run -- npm start  # decrypts in-memory, injects env vars, runs your command
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
- `noenvy run -- <command>` — Decrypt in-memory, inject as env vars, exec the command, never write plaintext to disk
- `noenvy add <KEY>` — Prompt for value, add to encrypted store
- `noenvy remove <KEY>` — Remove a key from the encrypted store
- `noenvy list` — Show keys (not values) currently stored
- `noenvy rotate` — Generate new encryption key, re-encrypt store

### Claude Code skill bootstrapper
- `noenvy install-skill` — Creates a skill in `~/.claude/skills/` (global, default) or `./.claude/skills/` (project-local) with confirmation prompt
- Skill teaches Claude Code how to use noenvy: when to suggest it, how to run commands with it, how to add secrets safely
- Default is global with prompt to confirm; flag to force project-local: `--project`

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

## Milestones

### Week 1: Foundation
- [ ] Repo scaffold with cobra
- [ ] `noenvy init` — reads `.env`, generates key, stores in keyring, writes `.noenvy`
- [ ] `noenvy run` — happy path: decrypt, inject, exec
- [ ] Cross-platform smoke tests (macOS + Linux at minimum; Windows if you have access)

### Week 2: Round out v1 commands
- [ ] `add`, `remove`, `list`, `rotate`
- [ ] `.gitignore` handling on init
- [ ] Walk-up directory lookup for `.noenvy`
- [ ] Good error messages (no `.noenvy` found, keyring unavailable, decryption failed, etc.)

### Week 3: Claude Code skill + polish
- [ ] `noenvy install-skill` with global/project prompts
- [ ] Write the skill markdown — what it teaches Claude Code about noenvy
- [ ] README with the top section above, demo GIF or asciinema cast
- [ ] CONTRIBUTING.md, LICENSE (MIT), CI for tests + releases

### Week 4: Ship
- [ ] GoReleaser config for cross-platform binaries (macOS arm64/amd64, Linux amd64/arm64, Windows amd64)
- [ ] Homebrew tap (`brew install noenvy` is table stakes)
- [ ] Launch post: Hacker News (Tue/Wed morning Eastern), r/programming, r/golang, LinkedIn
- [ ] Reach out directly to 5 people who'd find it useful

---

## Launch checklist

### Before posting
- [ ] README top section sings (under 100 words, demo visible immediately)
- [ ] Installation works on a fresh machine (test on a VM or borrowed laptop)
- [ ] At least one short demo (GIF or asciinema) in the README
- [ ] Threat model section is honest and clear
- [ ] Comparison table to alternatives: dotenv-vault, doppler, infisical, 1Password CLI, direnv
- [ ] License chosen (MIT recommended for max adoption)
- [ ] CI green, tests pass, `go vet` clean

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

## Naming check (do before any code)

- [ ] `noenvy` npm package name available
- [ ] `noenvy` available on Homebrew
- [ ] GitHub org/repo `noenvy` available
- [ ] Domain `noenvy.dev` or `noenvy.sh` available (nice-to-have, not required)
- [ ] No trademark conflicts (quick USPTO search)

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

Put these in `ROADMAP.md` after v1 ships and let user demand prioritize them.

---

## Success criteria

**Minimum win:** Repo exists, works, README is clean. You can point to it in any consulting or job conversation as evidence of your taste and judgment. This is achieved on day one of launch.

**Real win:** 200-500 GitHub stars within 3 months, a handful of issues from real users, one or two contributors. Sufficient signal to legitimately put in your LinkedIn headline.

**Big win:** Steady organic growth, picked up by a newsletter or two, becomes the default suggestion when someone asks "how should I handle local secrets." This compounds over years.

The minimum win is what matters. Everything above it is upside.
