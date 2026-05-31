#!/usr/bin/env bash
#
# Wrap the GoReleaser-built universal macOS binary in a double-clickable
# Toy Box.app, sign it with Developer ID (hardened runtime + the
# disable-library-validation entitlement binsdl needs), notarize + staple both
# the app and a .dmg, and upload the .dmg to the GitHub release.
#
# Runs on a macOS runner after GoReleaser (which leaves the universal binary in
# dist/ and creates the release). No-ops gracefully if signing secrets are
# absent, so unsigned/local runs still succeed.
#
# Usage: scripts/package-macos.sh <version>   # e.g. 0.1.4 (no leading v)
# Env: MACOS_SIGN_P12, MACOS_SIGN_PASSWORD, MACOS_NOTARY_KEY,
#      MACOS_NOTARY_KEY_ID, MACOS_NOTARY_ISSUER_ID, GH_TOKEN
set -euo pipefail

VERSION="${1:?usage: package-macos.sh <version>}"
TAG="v${VERSION}"

if [[ -z "${MACOS_SIGN_P12:-}" ]]; then
  echo "MACOS_SIGN_P12 not set — skipping signed .app/.dmg packaging."
  exit 0
fi

BIN="$(find dist -type f -name toybox -path '*universal*' | head -1)"
[[ -n "$BIN" ]] || { echo "could not find the universal binary under dist/"; exit 1; }
echo "universal binary: $BIN"

WORK="$(mktemp -d)"
APP="$WORK/Toy Box.app"
DMG="dist/toybox_${VERSION}_macos.dmg"
ENTITLEMENTS="entitlements.plist"

# --- build the .app bundle ---
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BIN" "$APP/Contents/MacOS/toybox"
chmod +x "$APP/Contents/MacOS/toybox"
cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleName</key><string>Toy Box</string>
  <key>CFBundleDisplayName</key><string>Toy Box</string>
  <key>CFBundleIdentifier</key><string>com.anthonyrego.toybox</string>
  <key>CFBundleVersion</key><string>${VERSION}</string>
  <key>CFBundleShortVersionString</key><string>${VERSION}</string>
  <key>CFBundleExecutable</key><string>toybox</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>NSHighResolutionCapable</key><true/>
</dict></plist>
PLIST

# --- import the Developer ID cert into a throwaway keychain ---
KEYCHAIN="$WORK/signing.keychain-db"
KEYCHAIN_PW="$(uuidgen)"
CERT="$WORK/cert.p12"
printf '%s' "$MACOS_SIGN_P12" | base64 --decode > "$CERT"
security create-keychain -p "$KEYCHAIN_PW" "$KEYCHAIN"
security set-keychain-settings -lut 21600 "$KEYCHAIN"
security unlock-keychain -p "$KEYCHAIN_PW" "$KEYCHAIN"
security import "$CERT" -P "$MACOS_SIGN_PASSWORD" -A -t cert -f pkcs12 -k "$KEYCHAIN"
security set-key-partition-list -S apple-tool:,apple: -k "$KEYCHAIN_PW" "$KEYCHAIN" >/dev/null
security list-keychain -d user -s "$KEYCHAIN" "login.keychain-db"
IDENTITY="$(security find-identity -v -p codesigning "$KEYCHAIN" | awk '/Developer ID Application/{print $2; exit}')"
[[ -n "$IDENTITY" ]] || { echo "no Developer ID Application identity in the cert"; exit 1; }
echo "signing identity: $IDENTITY"

# --- App Store Connect API key for notarization ---
KEY_P8="$WORK/notary.p8"
printf '%s' "$MACOS_NOTARY_KEY" | base64 --decode > "$KEY_P8"
notarize() { # notarize.sh <path-to-zip-or-dmg>
  xcrun notarytool submit "$1" --key "$KEY_P8" --key-id "$MACOS_NOTARY_KEY_ID" \
    --issuer "$MACOS_NOTARY_ISSUER_ID" --wait
}

# --- sign + notarize + staple the app ---
codesign --force --options runtime --timestamp \
  --entitlements "$ENTITLEMENTS" --sign "$IDENTITY" "$APP"
codesign --verify --strict --verbose=2 "$APP"
ditto -c -k --keepParent "$APP" "$WORK/app.zip"
notarize "$WORK/app.zip"
xcrun stapler staple "$APP"

# --- build, sign, notarize, staple the .dmg ---
STAGE="$WORK/dmg"
mkdir -p "$STAGE"
cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"
hdiutil create -volname "Toy Box" -srcfolder "$STAGE" -ov -format UDZO "$DMG"
codesign --force --timestamp --sign "$IDENTITY" "$DMG"
notarize "$DMG"
xcrun stapler staple "$DMG"

echo "=== final verification ==="
spctl --assess --type open --context context:primary-signature -vv "$DMG" || true
xcrun stapler validate "$DMG"

# --- attach to the release ---
gh release upload "$TAG" "$DMG" --clobber
echo "uploaded $DMG to release $TAG"
