# Release Signing (Self-Update Integrity)

DeckTap only installs a self-update whose `checksums.txt` carries a valid **Ed25519** signature from the release signing key. The public half is compiled into the binary (`updater.ReleasePublicKey` in `internal/updater/signature.go`). Anything that doesn't verify is refused, so a compromised GitHub account or release workflow can't push code to installed copies.

## How it works

1. The release workflow builds the packages and writes `checksums.txt` (`sha256sum` of every asset).
2. The `checksums` job runs `go run ./cmd/release-sign sign`. It signs this exact message:

   ```
   decktap-release-checksums-v1\n<release tag>\n<checksums.txt bytes>
   ```

   The result is published as `checksums.txt.sig` (base64).
3. On update, DeckTap downloads `checksums.txt` and `checksums.txt.sig` **first**. It verifies the signature for the tag it's about to install, and only then downloads the package and compares its SHA-256 with the signed value.

The update **fails closed** if:
- no public key is embedded (the build opens the release page instead)
- `checksums.txt` or `checksums.txt.sig` is missing
- the signature is invalid
- the signature belongs to another tag, which blocks replaying an old release as a "newer" one
- the asset isn't listed in the signed manifest
- the package's hash doesn't match

## One-time setup

Run these on a trusted machine (not CI):

```bash
go run ./cmd/release-sign keygen -out decktap-signing.key
```

The command prints the **public key** only. Then:

```bash
gh secret set DECKTAP_UPDATE_SIGNING_KEY --repo <owner>/DeckTap < decktap-signing.key
```

1. Paste the printed public key into `ReleasePublicKey` and open a PR.
2. Move `decktap-signing.key` to offline storage (for example a password manager vault restricted to release maintainers), then delete the local file.

Until a key is embedded, self-update is disabled and DeckTap opens the release page for a manual download. Until the secret is set, the release workflow **fails** at "Sign checksums" instead of publishing an unsigned release.

## Rotation

Installed copies trust only the key compiled into them:

1. Generate a new key, but don't replace the secret yet.
2. Ship one release that embeds the **new** public key and is still signed with the **old** key, so existing installs accept it.
3. Replace `DECKTAP_UPDATE_SIGNING_KEY` with the new key. Later releases are signed with it.

## If the key leaks

There is no remote revocation. Installed copies trust the embedded key until they update. Rotate immediately (steps above), publish a security advisory, and ask users to update. Keep the key offline to make this unlikely, and limit who can edit repository secrets.

This scheme authenticates the **auto-update** path. It is separate from OS code signing (Apple Developer ID / notarization, Windows Authenticode), which covers first installs. See Phase 1 in [`strategy/04-plan.md`](strategy/04-plan.md).
