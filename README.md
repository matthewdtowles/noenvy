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

### macOS — Homebrew

```bash
brew install matthewdtowles/tap/noenvy
```

### Linux — `.deb` / `.rpm` / `.apk`

Download the package for your distro from the [latest release](https://github.com/matthewdtowles/noenvy/releases/latest) and install:

```bash
# Debian / Ubuntu / Mint / Pop!_OS / Kali
sudo dpkg -i noenvy_*_linux_amd64.deb

# Fedora / RHEL / CentOS / Rocky
sudo rpm -i noenvy_*_linux_amd64.rpm

# Alpine
sudo apk add --allow-untrusted noenvy_*_linux_amd64.apk
```

Replace `amd64` with `arm64` if you're on ARM.

### Windows — direct download

Grab `noenvy_*_windows_x86_64.zip` from the [latest release](https://github.com/matthewdtowles/noenvy/releases/latest), extract, and put `noenvy.exe` somewhere on your `PATH`.

### Direct binary download (any platform)

If a package isn't available for your setup, grab the tarball/zip from the [latest release](https://github.com/matthewdtowles/noenvy/releases/latest), extract, and move the binary onto your `PATH`.

On macOS, the binary is currently unsigned. The first time you run a downloaded binary you'll hit Gatekeeper. Either right-click → Open in Finder, or strip the quarantine bit once:

```bash
xattr -d com.apple.quarantine ./noenvy
```

(Brew installs sidestep this — Homebrew handles the quarantine attribute itself.)

### From source

```bash
go install github.com/matthewdtowles/noenvy@latest
```

Or:

```bash
git clone https://github.com/matthewdtowles/noenvy.git
cd noenvy
go build -o noenvy .
```

Requires Go 1.22+ to build.

### Platform support

noenvy uses the operating system's secure credential store for the encryption key. Platform support depends on whether that store is available:

| Platform | Credential store | Status |
|---|---|---|
| macOS | Keychain | ✅ Works out of the box |
| Windows | Credential Manager | ✅ Works out of the box |
| Linux desktop (GNOME / KDE / similar) | Secret Service (gnome-keyring or KWallet) | ✅ Works when the keyring daemon is running (default in most desktop distros) |
| Headless Linux servers | None by default | ❌ Not supported in v1 |
| Docker containers | None by default | ❌ Not supported in v1 |
| WSL2 (Linux on Windows) | None by default | ❌ Not supported in v1 unless you install and run a Secret Service implementation manually |
| Devcontainers / Codespaces / remote SSH dev | None by default | ❌ Not supported in v1 |

If you hit `failed to access keyring` or similar errors on Linux, your system likely doesn't have a Secret Service implementation running. A passphrase-protected file backend is on the roadmap for headless and WSL2 cases.

## Usage

### `noenvy init`

Reads `.env` from the current directory, generates a 32-byte AES key, stores the key in your OS keyring, and writes an encrypted file to `~/.noenvy/projects/<project-id>`. Adds `.env` to `.gitignore`.

```bash
noenvy init
# Encrypted .env → /Users/you/.noenvy/projects/46655b8ade8b84e89b391fd6258f85e8
# Key stored in OS keyring (service=noenvy, project=46655b8a…)
# Added to .gitignore: .env
```

The encrypted file lives in your home directory by default. The project it belongs to is identified by the project root path (detected by walking up for `.git`, `package.json`, `Cargo.toml`, `go.mod`, or `pyproject.toml`).

Flags:

- `--env-file <path>` — use a different source file (default `.env`)
- `--project` — store the encrypted file as `.noenvy` inside the project directory instead of under `~/.noenvy`. In this mode `.noenvy` is also added to `.gitignore`.
- `--force` — overwrite any existing encrypted file and replace the keyring entry

The encrypted file is **not designed to be committed** even though it would be safe to. Without the encryption key (which lives only in your local OS keyring and cannot be shared in v1), the file is just opaque bytes. Team sharing is out of scope for v1.

### `noenvy run -- <command>`

Walks up from the current directory to find the project root, locates the encrypted file (centralized or `--project` mode — either works), decrypts it in memory with the keyring key, and exec's the given command with those secrets in its environment.

```bash
noenvy run -- npm start
noenvy run -- pytest
noenvy run -- env | grep API_KEY
```

The `--` separator is recommended but optional — `noenvy run npm start` works too. Variables from the encrypted file override any same-named variables in the parent environment.

Exit code from the child command is propagated.

## How it works

**File format** (encrypted file, regardless of layout):

```
byte 0       version (currently 0x01)
bytes 1..12  12-byte AES-GCM nonce
bytes 13..N  ciphertext + 16-byte AES-GCM authentication tag
```

AES-256-GCM via Go's `crypto/aes` and `crypto/cipher` stdlib. Nonce is generated per encryption from `crypto/rand`. Tamper detection is built in via GCM's authentication tag — modifying any byte of the file causes decryption to fail.

**Storage layout (default — centralized):**

```
~/.noenvy/
└── projects/
    └── <project-id>   # one encrypted file per project
```

`<project-id>` is the SHA-256 of the project root's absolute path, hex-encoded, first 32 chars.

**Storage layout (`--project` mode):**

The encrypted file lives at `<project-root>/.noenvy` instead, and `.noenvy` is added to `.gitignore` alongside `.env`. Useful if you prefer keeping per-project files inside the project directory.

**Key storage:** Encryption keys live in the OS keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service) under service name `noenvy` and an account name equal to the project ID. Keys never touch disk.

**Project root detection:** `noenvy init` and `noenvy run` both walk up from the current directory looking for one of `.git`, `package.json`, `Cargo.toml`, `go.mod`, or `pyproject.toml`. First match wins (innermost project takes precedence — important for monorepos). If no marker is found, the current directory is used as the project root.

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
