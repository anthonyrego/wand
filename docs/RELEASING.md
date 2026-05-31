# Releasing

Releases are built by GoReleaser in GitHub Actions (`.github/workflows/release.yml`)
whenever a `v*` tag is pushed:

```sh
git tag v0.1.0
git push origin v0.1.0
```

That cross-compiles the `toybox` app for macOS (one universal Intel + Apple
Silicon binary), Windows, and Linux (amd64 + arm64), then publishes a GitHub
Release with the archives + `checksums.txt`.

The job runs on a **macOS runner** on purpose: a darwin binary cross-built on
Linux is a malformed Mach-O that crashes under Rosetta, and code-signing /
notarization need to run there. macOS still cross-compiles the Linux/Windows
targets (the app is pure Go).

Dry-run the whole build locally (no signing, no publish):

```sh
make release-snapshot   # outputs to dist/
```

## macOS signing & notarization (one-time setup)

Without the secrets below, releases still succeed but the macOS binary ships
**unsigned** — Gatekeeper blocks it ("unidentified developer"), and on Apple
Silicon an unsigned binary is killed outright. Setting them up makes the
download "just open."

You need a paid **Apple Developer account** ($99/yr). Then create two things and
store five GitHub Actions secrets (repo → Settings → Secrets and variables →
Actions → New repository secret).

### 1. Developer ID Application certificate → `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`

1. In Xcode (Settings → Accounts → Manage Certificates → +) or the
   [Developer portal](https://developer.apple.com/account/resources/certificates),
   create a **Developer ID Application** certificate (the kind for distributing
   apps *outside* the App Store — not "Apple Development"/"Apple Distribution").
2. In **Keychain Access**, find that certificate, expand it so its private key
   shows, select both, right-click → **Export 2 items…** → save as `cert.p12`
   and set an export password.
3. Base64-encode it and copy:
   ```sh
   base64 -i cert.p12 | pbcopy
   ```
   - `MACOS_SIGN_P12` = that base64 string
   - `MACOS_SIGN_PASSWORD` = the export password from step 2

### 2. App Store Connect API key (for the Notary service) → `MACOS_NOTARY_*`

1. In [App Store Connect](https://appstoreconnect.apple.com) → **Users and
   Access → Integrations → App Store Connect API**, generate a **Team key** with
   at least the **Developer** role. Download the `AuthKey_XXXX.p8` (you can only
   download it once).
2. Note the **Key ID** (next to the key) and the **Issuer ID** (top of the page).
3. Base64-encode the key:
   ```sh
   base64 -i AuthKey_XXXX.p8 | pbcopy
   ```
   - `MACOS_NOTARY_KEY` = that base64 string
   - `MACOS_NOTARY_KEY_ID` = the Key ID
   - `MACOS_NOTARY_ISSUER_ID` = the Issuer ID

### Summary of secrets

| Secret | Value |
| --- | --- |
| `MACOS_SIGN_P12` | base64 of the Developer ID Application `.p12` |
| `MACOS_SIGN_PASSWORD` | the `.p12` export password |
| `MACOS_NOTARY_KEY` | base64 of the App Store Connect `.p8` |
| `MACOS_NOTARY_KEY_ID` | the API key's Key ID |
| `MACOS_NOTARY_ISSUER_ID` | the API key's Issuer ID |

Once these exist, the next `v*` tag produces a signed, notarized, stapled macOS
universal binary that opens with no security prompt.

## macOS .app / .dmg

A bare unix binary can't be double-clicked in Finder ("not an app"), so after
GoReleaser runs, `scripts/package-macos.sh` wraps the universal binary in a
**Toy Box.app**, signs it (Developer ID + hardened runtime + the
`disable-library-validation` entitlement that the embedded SDL3 needs),
notarizes + staples both the app and a `.dmg`, and uploads the `.dmg` to the
release. It uses the same five secrets above — no extra setup — and no-ops if
they're absent.

## Windows & Linux

- **Windows** binaries are unsigned (SmartScreen will warn on first run). Code
  signing needs a separate (paid) Windows certificate; not set up.
- **Linux/amd64** bundles SDL3 (self-contained). **Linux/arm64** (Raspberry Pi)
  needs system SDL3 at runtime (`sudo apt install libsdl3-0`).
