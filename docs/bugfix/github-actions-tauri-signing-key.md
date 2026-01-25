# GitHub Actions: Tauri Signing Key Password Issue

## Problem

When using Tauri's updater signing in GitHub Actions, builds fail with:

```
failed to decode secret key: incorrect updater private key password: Wrong password for that key
```

This happens even when:
- The signing key was generated with `--ci` flag (empty password)
- The same key works perfectly in local builds
- The `TAURI_SIGNING_PRIVATE_KEY` secret contains the correct key

## Root Cause

GitHub Actions does not properly handle empty string passwords for Tauri signing keys.

We tried multiple approaches that all failed:

```yaml
# Approach 1: Empty string in YAML env section
env:
  TAURI_SIGNING_PRIVATE_KEY_PASSWORD: ""

# Approach 2: Shell export with double quotes
run: |
  export TAURI_SIGNING_PRIVATE_KEY_PASSWORD=""

# Approach 3: Shell export with single quotes
run: |
  export TAURI_SIGNING_PRIVATE_KEY_PASSWORD=''

# Approach 4: Referencing non-existent secret (hoping for empty)
env:
  TAURI_SIGNING_PRIVATE_KEY_PASSWORD: ${{ secrets.TAURI_SIGNING_PRIVATE_KEY_PASSWORD }}
```

None of these correctly pass an empty string to the Tauri CLI. GitHub Actions either:
- Treats `""` as undefined/null
- Adds invisible characters
- Handles the empty value inconsistently

## Solution

**Use an actual password instead of empty string.**

1. Generate a new key with an explicit password:
   ```bash
   bun tauri signer generate --password YOUR_PASSWORD -w ~/.config/maily/update.key
   ```

2. Update `tauri.conf.json` with the new public key:
   ```json
   {
     "plugins": {
       "updater": {
         "pubkey": "<content of update.key.pub>"
       }
     }
   }
   ```

3. Set GitHub secrets:
   - `TAURI_SIGNING_PRIVATE_KEY`: Content of `update.key`
   - `TAURI_SIGNING_PRIVATE_KEY_PASSWORD`: Your password

4. Update workflow:
   ```yaml
   - name: Build Tauri app
     env:
       TAURI_SIGNING_PRIVATE_KEY: ${{ secrets.TAURI_SIGNING_PRIVATE_KEY }}
       TAURI_SIGNING_PRIVATE_KEY_PASSWORD: ${{ secrets.TAURI_SIGNING_PRIVATE_KEY_PASSWORD }}
     run: |
       bun tauri build --target aarch64-apple-darwin
   ```

## Key Takeaways

- **Don't use `--ci` flag** for keys used in GitHub Actions
- **Always use a real password** - GitHub Actions handles non-empty secrets reliably
- **Local builds are not indicative** of GitHub Actions behavior for empty strings
- The Tauri key is always "encrypted" - even with `--ci`, it's encrypted with empty string password

## Related Files

- `.github/workflows/release-desktop-aarch64.yml`
- `.github/workflows/release-desktop-x86_64.yml`
- `tauri/src-tauri/tauri.conf.json`
