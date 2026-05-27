#!/usr/bin/env bash
# Generates the GPG signing key used to sign the apt repository, and prints
# the values you need to paste into GitHub Actions secrets.
#
# Run this ONCE on a trusted local machine. The private key never leaves
# your machine except as a GitHub Actions secret. The public key gets
# committed to the gh-pages branch so users can verify packages.
#
# Re-running this script will create a SECOND key alongside the first.
# Don't do that unless you're intentionally rotating.

set -euo pipefail

KEY_NAME="noenvy apt repo signing key"
KEY_EMAIL="matthewdtowles+noenvy-apt@gmail.com"
OUT_DIR="$(cd "$(dirname "$0")/.." && pwd)/.apt-signing-key"

if ! command -v gpg >/dev/null 2>&1; then
  echo "error: gpg is not installed" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"
chmod 700 "$OUT_DIR"

if [ -f "$OUT_DIR/private.asc" ]; then
  echo "error: $OUT_DIR/private.asc already exists. Refusing to overwrite." >&2
  echo "If you intend to rotate, delete that directory first." >&2
  exit 1
fi

echo "Enter a passphrase for the new signing key (you will need it as a GitHub secret):"
read -rs PASSPHRASE
echo

if [ -z "$PASSPHRASE" ]; then
  echo "error: passphrase cannot be empty" >&2
  exit 1
fi

BATCH_FILE="$(mktemp)"
GNUPGHOME="$(mktemp -d)"
export GNUPGHOME
# Combined trap up front so the passphrase-bearing BATCH_FILE is always cleaned
# up no matter where the script exits, including future edits that add work
# between resource creation and key generation.
trap 'rm -rf "$GNUPGHOME"; rm -f "$BATCH_FILE"' EXIT

cat >"$BATCH_FILE" <<EOF
%echo Generating signing key (this can take a minute)
Key-Type: RSA
Key-Length: 4096
Key-Usage: sign
Name-Real: $KEY_NAME
Name-Email: $KEY_EMAIL
Expire-Date: 2y
Passphrase: $PASSPHRASE
%commit
%echo Done
EOF

gpg --batch --generate-key "$BATCH_FILE"

FINGERPRINT="$(gpg --list-secret-keys --with-colons | awk -F: '/^fpr:/ {print $10; exit}')"

gpg --batch --yes --pinentry-mode loopback --passphrase "$PASSPHRASE" \
    --armor --export-secret-keys "$FINGERPRINT" >"$OUT_DIR/private.asc"
gpg --armor --export "$FINGERPRINT" >"$OUT_DIR/public.asc"

chmod 600 "$OUT_DIR/private.asc"
chmod 644 "$OUT_DIR/public.asc"

cat <<EOF

Key generated.

Fingerprint: $FINGERPRINT

Files written to: $OUT_DIR
  private.asc — secret key (DO NOT COMMIT, DO NOT SHARE)
  public.asc  — public key (will be committed to gh-pages by the workflow)

Next steps:

1. Add the private key as a GitHub Actions secret named GPG_PRIVATE_KEY:
     gh secret set GPG_PRIVATE_KEY < $OUT_DIR/private.asc

2. Add the passphrase as a GitHub Actions secret named GPG_PASSPHRASE:
     gh secret set GPG_PASSPHRASE
     (paste the passphrase when prompted)

3. Add the fingerprint as a GitHub Actions secret named GPG_FINGERPRINT:
     echo $FINGERPRINT | gh secret set GPG_FINGERPRINT

4. After secrets are set, you can (and should) delete the local key material:
     rm -rf $OUT_DIR

5. Update the FINGERPRINT shown in README.md with the value above so users
   can verify the public key they download.

EOF
