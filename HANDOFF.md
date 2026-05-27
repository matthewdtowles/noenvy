# Apt repository — one-time setup

This branch adds a signed apt repository served from the `gh-pages` branch via GitHub Pages. After this setup, end users install with:

```bash
sudo install -d -m 0755 /etc/apt/keyrings
curl -fsSL https://matthewdtowles.github.io/noenvy/key.gpg \
  | sudo gpg --dearmor -o /etc/apt/keyrings/noenvy.gpg
echo "deb [signed-by=/etc/apt/keyrings/noenvy.gpg] https://matthewdtowles.github.io/noenvy stable main" \
  | sudo tee /etc/apt/sources.list.d/noenvy.list
sudo apt update && sudo apt install noenvy
```

To make the above actually work, run through the steps below once.

## 1. Generate the GPG signing key (local)

```bash
./scripts/setup-apt-signing-key.sh
```

Pick a strong passphrase and store it somewhere durable (1Password / your password manager). You'll need it again if you ever rotate the key or manually re-run the publishing workflow. The script writes `private.asc`, `public.asc`, and prints the key fingerprint. Output lives at `~/.local/share/noenvy-apt-signing-key/` (outside the repo, so it can't be backed up, indexed, or accidentally committed).

## 2. Add GitHub Actions secrets

The script prints these commands with the right paths. Run them in the repo root:

```bash
gh secret set GPG_PRIVATE_KEY < ~/.local/share/noenvy-apt-signing-key/private.asc
gh secret set GPG_PASSPHRASE         # paste the passphrase when prompted
echo <FINGERPRINT> | gh secret set GPG_FINGERPRINT
```

Verify with `gh secret list` — you should see all three.

## 3. Delete the local key material

```bash
rm -rf ~/.local/share/noenvy-apt-signing-key
```

The private key now lives only in GitHub Actions secrets. The public key will be republished to `gh-pages` on every release.

## 4. Paste the fingerprint into README.md

Find this line in `README.md`:

```
# Expected fingerprint: <set me after running scripts/setup-apt-signing-key.sh>
```

Replace the placeholder with the fingerprint from step 1. This is the value users will check against when verifying the key out-of-band.

## 5. Bootstrap the `gh-pages` branch

The workflow can create it, but it's cleaner to seed it first so GitHub Pages has something to point at:

```bash
git switch --orphan gh-pages
git commit --allow-empty -m "init gh-pages"
git push -u origin gh-pages
git switch apt-repo
```

## 6. Enable GitHub Pages

GitHub → repo Settings → Pages
- Source: **Deploy from a branch**
- Branch: **gh-pages** / **(root)**
- Save

Wait for the first deploy to go green.

## 7. Merge this PR to main

`gh workflow run` (and the GitHub Actions UI's "Run workflow" button) only see workflow files that exist on the **default branch**. The workflow can't be triggered from `apt-repo` even with `--ref` — it has to land on `main` first. After CI passes (the fingerprint check from step 4 unblocks it), merge PR #2.

## 8. Publish the apt repo for the existing v0.0.1 release

After merge, switch back to a clean checkout of main and backfill v0.0.1:

```bash
git switch main && git pull
gh workflow run apt-repo.yml -f tag=v0.0.1
gh run watch
```

When it finishes, `https://matthewdtowles.github.io/noenvy/key.gpg` and `https://matthewdtowles.github.io/noenvy/dists/stable/Release` should both load.

Future tagged releases auto-publish via the `release: published` trigger — no manual dispatch needed.

## 9. Smoke-test the install path

On a fresh Ubuntu VM (or this machine):

```bash
sudo install -d -m 0755 /etc/apt/keyrings
curl -fsSL https://matthewdtowles.github.io/noenvy/key.gpg \
  | sudo gpg --dearmor -o /etc/apt/keyrings/noenvy.gpg
echo "deb [signed-by=/etc/apt/keyrings/noenvy.gpg] https://matthewdtowles.github.io/noenvy stable main" \
  | sudo tee /etc/apt/sources.list.d/noenvy.list
sudo apt update
sudo apt install noenvy
noenvy --version
```

If `apt update` complains about a missing/invalid signature, re-check that `GPG_FINGERPRINT` matches the key actually loaded into the workflow. If something else goes wrong, fix forward with new commits to `main`.

## Key rotation (future)

To rotate the signing key:

1. Re-run `./scripts/setup-apt-signing-key.sh` (delete `~/.local/share/noenvy-apt-signing-key/` first).
2. Update all three GitHub secrets with the new values.
3. Update the fingerprint in `README.md`.
4. Re-run the apt-repo workflow against any tag so the new public key is published.
5. Notify users — they will need to re-import the key:
   ```bash
   curl -fsSL https://matthewdtowles.github.io/noenvy/key.gpg \
     | sudo gpg --dearmor -o /etc/apt/keyrings/noenvy.gpg
   ```

## Delete this file once setup is done

This handoff doc isn't useful after the one-time setup. Delete `HANDOFF.md` before merging, or leave the deletion as part of the merge commit.
