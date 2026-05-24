# noenvy

**Stop leaving secrets in .env files.**

Encrypts your `.env` into a `.noenvy` file using a key stored in your OS keyring. Run any command with secrets injected as environment variables. No accounts, no servers, no plaintext on disk.

```bash
noenvy init              # encrypts .env, stores key in keyring
noenvy run -- npm start  # decrypts in-memory, injects env vars, runs your command
```

Works with any language because it runs at the process boundary, not in your code.

---

## Why noenvy

Most developers handle local secrets badly:

- Plaintext `.env` files sit on disk and get accidentally committed.
- Cloud secret managers (Doppler, Infisical, AWS Secrets Manager) are overkill for local dev and require accounts and network.
- Per-language dotenv libraries load secrets into the process where any dependency can read them.
- Tools like 1Password CLI require a paid account.

noenvy is for developers who want their local secrets handled properly with zero infrastructure and zero accounts.

## Threat model

Being explicit about this matters more than it sounds. noenvy uses the same threat model as every other local secret tool — saying so plainly builds trust rather than papering over it.

**Protects against:**

- Plaintext secrets sitting on disk.
- Accidental `git commit` of secret files.
- Casual filesystem access by other processes that aren't running as your user.

**Does NOT protect against:**

- A compromised user session — an attacker with your login has your OS keyring.
- Malicious code running as your user (npm dep, brew package, browser extension, etc.).
- Memory inspection of the child process while it runs with secrets in its environment.

If your local machine is compromised, noenvy won't save you. Neither will any other local secret tool. For team-scale secret management, look at Vault, Doppler, or Infisical.

## Install

> Pre-release. Binary releases and a Homebrew tap are part of the v1 launch.

For now:

```bash
go install github.com/matthewdtowles/noenvy@latest
```

Or build from source:

```bash
git clone https://github.com/matthewdtowles/noenvy.git
cd noenvy
go build -o noenvy .
```

Requires Go 1.22+ to build.

## Usage

### `noenvy init`

Reads `.env` from the current directory, generates a 32-byte AES key, stores it in your OS keyring, and writes an encrypted `.noenvy` file. Also appends `.env` and `.noenvy` to `.gitignore` so neither is accidentally committed.

```bash
noenvy init
# Encrypted .env → .noenvy
# Key stored in OS keyring (service=noenvy, project=415ea1da76640376e5472fd293108a75)
# Added to .gitignore: .env, .noenvy
```

Flags:

- `--env-file <path>` — use a different source file (default `.env`)
- `--force` — overwrite an existing `.noenvy` and replace the keyring entry

If you want to commit the encrypted `.noenvy` file (so collaborators with the key can use it), remove that line from `.gitignore` yourself.

### `noenvy run -- <command>`

Walks up from the current directory to find a `.noenvy` file, looks up its encryption key in the OS keyring, decrypts the secrets in memory, and exec's the given command with those secrets in its environment.

```bash
noenvy run -- npm start
noenvy run -- pytest
noenvy run -- env | grep API_KEY
```

The `--` separator is recommended but optional — `noenvy run npm start` works too. Variables from `.noenvy` override any same-named variables in the parent environment.

Exit code from the child command is propagated.

## How it works

**File format** (`.noenvy`):

```
byte 0       version (currently 0x01)
bytes 1..12  12-byte AES-GCM nonce
bytes 13..N  ciphertext + 16-byte AES-GCM authentication tag
```

AES-256-GCM via Go's `crypto/aes` and `crypto/cipher` stdlib. Nonce is generated per encryption from `crypto/rand`. Tamper detection is built in via GCM's authentication tag — modifying any byte of the file causes decryption to fail.

**Key storage:** Keys live in the OS keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service) under service name `noenvy` and an account name derived from the absolute path of the `.noenvy` file (SHA-256 hash, hex-encoded, first 32 chars). Keys never touch disk.

**Project lookup:** `noenvy run` walks up from the current directory like `git` looking for a `.noenvy` file, then derives the keyring account name from its path.

## Comparison to alternatives

- vs **dotenv-vault** — noenvy is local-only with no cloud sync. dotenv-vault syncs across machines via their service, which is useful if you need that and unnecessary friction if you don't.
- vs **Doppler / Infisical** — noenvy has no server and no account. Those are team-scale secret managers; noenvy is local-developer scale.
- vs **1Password CLI** — noenvy doesn't require a 1Password subscription.
- vs **direnv** — direnv loads plaintext `.envrc` files when you `cd` into a directory; nothing is encrypted. noenvy encrypts at rest.
- vs **language-specific dotenv libraries** — those load secrets into your process where any dependency can read them via `process.env` / `os.environ`. noenvy runs at the process boundary, so your application code can still read env vars but the secrets never sit in plaintext on disk.

## Roadmap

Deliberately **not** in v1, to keep the tool sharp:

- Sync across machines (different product).
- Team sharing of encrypted secrets (Vault / Doppler territory).
- Cloud secret manager backends (AWS, GCP, Vault as alternative key stores).
- Multi-environment profiles (`.env.staging`, `.env.prod`).
- Arbitrary file format support beyond `.env`.
- TUI for managing secrets.
- Editor or IDE integrations.
- Pre-commit hook to detect leaked secrets.

If any of these become must-haves for real users after launch, they'll be evaluated for v2.

## Contributing

This is a small project that's still finding its shape. Issues and discussion are welcome; please open an issue before sending a PR with new features so we can talk about scope.

## License

MIT — see [LICENSE](LICENSE).
