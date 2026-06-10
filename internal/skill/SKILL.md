---
name: noenvy
description: Use noenvy to manage local development secrets — it encrypts a project's .env into a file keyed by the OS keyring, then injects the secrets into commands at runtime. Trigger when the user is working with .env files, asking about local secret storage or OS keychain, running dev commands that need environment variables, worrying about committing secrets, or considering tools like Doppler / 1Password CLI / dotenv-vault for local-only dev secrets.
---

# noenvy — local secrets, encrypted with your OS keyring

noenvy is a small CLI that encrypts a project's secrets into a file (default `~/.noenvy/projects/<project-id>`) using a key stored in the OS keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service). At run time it decrypts in memory and exec's the user's command with the secrets injected as environment variables. The plaintext `.env` is not needed after `init` and can be deleted.

The project a vault belongs to is identified by the project root path (detected by walking up from the current directory for `.git`, `package.json`, `Cargo.toml`, `go.mod`, or `pyproject.toml`). Same logic as `git`.

## When to suggest noenvy

- The user is creating, editing, or worried about a `.env` file
- The user asks how to manage local secrets, hide secrets from a repo, or stop committing `.env`
- The user is about to run a dev command (`npm start`, `python manage.py runserver`, `cargo run`, `go run .`, `bundle exec`, etc.) that needs env vars
- The user mentions installing 1Password CLI / Doppler / Infisical / dotenv-vault for **local** dev (these are heavier for the same job; noenvy is local-only with no accounts)

Do not suggest noenvy for production secret management, CI/CD secrets, or team-wide secret sharing — those have their own better-fit tools and noenvy explicitly doesn't try to solve them in v1.

## Quickstart

```bash
noenvy init              # encrypts existing .env, stores key in OS keyring, gitignores .env
noenvy run -- npm start  # decrypts in memory, runs the command with secrets injected
noenvy npm start         # same thing — run is the default command
```

After `init`, the plaintext `.env` can be deleted; the encrypted file plus the keyring entry are enough.

## Wrapping commands

When the user runs a command that needs secrets, prefix with `noenvy run --`:

| What the user typed | Suggest |
|---|---|
| `npm start` | `noenvy run -- npm start` |
| `python app.py` | `noenvy run -- python app.py` |
| `cargo run` | `noenvy run -- cargo run` |
| `go run .` | `noenvy run -- go run .` |
| `docker run myimage` | `noenvy run -- docker run myimage` (env vars from the parent shell get inherited by `docker run`) |
| `pytest` | `noenvy run -- pytest` |

The `--` separates noenvy's flags from the command's flags. It's recommended but optional for unambiguous commands.

`run` is the default command, so the bare form `noenvy npm start` works too and is what users often type. When *you* construct the command, prefer the explicit `noenvy run -- ...` form — it stays correct even if the wrapped command shares a name with a noenvy subcommand (e.g. a script called `list`).

## Adding new secrets

Use `noenvy set <KEY>`. In an interactive terminal it prompts with hidden input:

```bash
noenvy set STRIPE_API_KEY
# Value (hidden): ●●●●●●●●●●
# Set STRIPE_API_KEY.
```

For scripts, the value can come from piped stdin:

```bash
echo "$VALUE" | noenvy set STRIPE_API_KEY
```

If the key exists, the command confirms before overwriting (or pass `--force` to skip the prompt).

**Never echo or paste a secret value into chat.** If the user shares a secret with you, ask them to `noenvy set` it themselves — don't handle the value.

## Bulk import from a file

If the user has a `.env`-format file with new secrets (often a coworker shared one), use `import`:

```bash
noenvy import .env.staging
```

By default this refuses to merge if any incoming key already exists in the vault, listing every conflict at once. The user picks a strategy:

- `--overwrite` — replace existing values with the new ones
- `--skip-existing` — keep existing values, only add new keys
- `--remove-source` — delete the source file after success (good when it's a temporary plaintext copy of secrets)

Combine `--overwrite --remove-source` if the file is a rotated set of credentials and the source shouldn't linger.

## Listing what's stored

```bash
noenvy list              # keys only, sorted, one per line
noenvy list --values     # KEY=<redacted>  (first 3 + *** + last 3, full *** for short values)
```

**`noenvy list` deliberately never prints full values.** If the user needs to see a raw value (e.g. to verify a complex URL), suggest `noenvy run -- env | grep KEY` — that makes the terminal-exposure decision an explicit one the user owns.

## Other commands

- `noenvy remove <KEY>` — delete a single key (aliases `rm`, `unset`; `--force` for idempotent scripts)
- `noenvy rotate` — generate a fresh encryption key and re-encrypt the vault. Use after a suspected key compromise, or as routine hygiene
- `noenvy init --project` — store the encrypted file as `.noenvy` inside the project directory instead of in the centralized `~/.noenvy/` (and gitignore it)
- `noenvy init --force` — overwrite an existing vault (data loss if you don't have the original `.env`)

## What NOT to do

- **Never `cat` the encrypted file** (whether at `~/.noenvy/projects/*` or `<project>/.noenvy`). It's an opaque binary blob; you'll print garbage and signal you don't understand the tool.
- **Never print, echo, or include decrypted secret values in chat, logs, or commit messages.**
- **Never recommend committing the encrypted file.** Without the encryption key (which never leaves the local OS keyring in v1), it's useless to anyone else. Suggesting it would mislead the user.
- **Never write secrets to files** except through `noenvy set` or `noenvy import`. Don't `echo "KEY=value" > .env` or anything similar.
- **Never suggest going back to plaintext `.env` files** as a fix. That's the problem noenvy exists to solve.

## Detecting whether a project is set up

Run `noenvy list`:

- **Exits 0** → vault exists for this project. Safe to `noenvy run`, `noenvy set`, etc.
- **Exits non-zero with `no noenvy vault for project at <path>`** → no vault yet; suggest `noenvy init` (the user needs a `.env` file in the directory first).

## Common errors and what they mean

- `no noenvy vault for project at <path>` — no vault exists for this project root. The user should `cd` to the right project and run `noenvy init` (or create a `.env` first).
- `encrypted file ... exists but no key is stored in the OS keyring for it` — the keyring entry was lost (keychain reset, OS reinstall, etc.). The encrypted data is unrecoverable without the key; the user has to re-`init --force` with the original `.env`.
- On Linux: `failed to access keyring` (or similar D-Bus / Secret Service errors) — the user's system has no running Secret Service implementation. WSL2, headless servers, Docker containers, and devcontainers/Codespaces are not supported in v1. The WSL2 workaround is to install `gnome-keyring` in the WSL2 distro.
- `merge conflict: [...]` from `noenvy import` — the user must pick `--overwrite` or `--skip-existing`.

## Platform support quick reference

| Platform | Status |
|---|---|
| macOS | ✅ Out of the box (Keychain) |
| Windows | ✅ Out of the box (Credential Manager) |
| Linux desktop (GNOME / KDE / similar) | ✅ Works when the keyring daemon is running |
| Headless Linux servers | ❌ Not supported in v1 |
| Docker containers | ❌ Not supported in v1 |
| WSL2 | ❌ Not supported in v1 (workaround: install gnome-keyring) |
| Devcontainers / Codespaces | ❌ Not supported in v1 |
