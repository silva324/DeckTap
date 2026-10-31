#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo "0.0.1")}"
CLEAN_VERSION="${VERSION#v}"
DIST_DIR="${ROOT_DIR}/dist"
APP_NAME="DeckTap"
APP_BUNDLE="${DIST_DIR}/${APP_NAME}.app"
DMG_NAME="DeckTap-${VERSION}-macOS-Universal.dmg"
TAR_NAME="DeckTap-${VERSION}-macOS-Universal.tar.gz"

echo "==> Building ${APP_NAME} ${VERSION} for macOS (Universal)..."
mkdir -p "${DIST_DIR}" build

# 1. Generate icon.icns from cmd/make-icon (always, so a stale local icon never ships)
echo "==> Generating icon.icns..."
rm -rf build/icon.iconset
go run ./cmd/make-icon build/icon.iconset
iconutil -c icns build/icon.iconset -o build/icon.icns

# 2. Build binaries (arm64 + amd64)
echo "==> Compiling darwin/arm64..."
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -buildvcs=false -ldflags "-s -w -X main.version=${VERSION}" -o "build/decktap-arm64" ./cmd/decktap

echo "==> Compiling darwin/amd64..."
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -buildvcs=false -ldflags "-s -w -X main.version=${VERSION}" -o "build/decktap-amd64" ./cmd/decktap

echo "==> Creating Universal Binary..."
lipo -create -output "build/decktap-universal" "build/decktap-arm64" "build/decktap-amd64"
chmod +x "build/decktap-universal"

# 3. Create .app Bundle
echo "==> Assembling ${APP_NAME}.app..."
rm -rf "${APP_BUNDLE}"
mkdir -p "${APP_BUNDLE}/Contents/MacOS"
mkdir -p "${APP_BUNDLE}/Contents/Resources"

cp "build/decktap-universal" "${APP_BUNDLE}/Contents/MacOS/DeckTap"
cp "build/icon.icns" "${APP_BUNDLE}/Contents/Resources/icon.icns"

cat <<EOF > "${APP_BUNDLE}/Contents/Info.plist"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>DeckTap</string>
    <key>CFBundleIdentifier</key>
    <string>com.silva324.decktap</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>${CLEAN_VERSION}</string>
    <key>CFBundleVersion</key>
    <string>${CLEAN_VERSION}</string>
    <key>CFBundleIconFile</key>
    <string>icon.icns</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSAppleEventsUsageDescription</key>
    <string>DeckTap requires permission to control Microsoft PowerPoint and read speaker notes.</string>
</dict>
</plist>
EOF

# 4. Ad-Hoc Sign App Bundle
echo "==> Ad-hoc signing ${APP_BUNDLE}..."
codesign --force --deep --sign - "${APP_BUNDLE}" 2>/dev/null || true

# 5. Create DMG
echo "==> Packaging DMG: ${DMG_NAME}..."
DMG_STAGING="${DIST_DIR}/dmg_staging"
rm -rf "${DMG_STAGING}" "${DIST_DIR}/${DMG_NAME}"
mkdir -p "${DMG_STAGING}"

cp -R "${APP_BUNDLE}" "${DMG_STAGING}/"
ln -s /Applications "${DMG_STAGING}/Applications"

cat << 'NOTE_EOF' > "${DMG_STAGING}/FIRST_LAUNCH_NOTE.txt"
======================================================================
DeckTap — macOS First Launch Guide (Gatekeeper Note)
======================================================================

DeckTap is free, community open-source software and is not signed
with a paid Apple Developer ID certificate ($99/year).

If macOS displays "Apple could not verify DeckTap.app is free of malware":

Method 1 (System Settings — Recommended):
  1. Drag DeckTap to Applications.
  2. Open "System Settings" > "Privacy & Security".
  3. Scroll down to the "Security" section.
  4. Click "Open Anyway" next to the DeckTap notification.

Method 2 (Right-Click):
  1. Drag DeckTap to Applications.
  2. Right-click (or Control-click) "DeckTap.app" in Finder.
  3. Select "Open", then click "Open" in the security prompt.

Method 3 (Terminal):
  Run: xattr -cr /Applications/DeckTap.app

(macOS only asks once on first launch!)
======================================================================
NOTE_EOF

hdiutil create -volname "${APP_NAME}" \
    -srcfolder "${DMG_STAGING}" \
    -ov -format UDZO \
    "${DIST_DIR}/${DMG_NAME}"

rm -rf "${DMG_STAGING}"

# 6. Create Standalone Universal Tarball
echo "==> Packaging Tarball: ${TAR_NAME}..."
(
    cd "build"
    tar -czf "${DIST_DIR}/${TAR_NAME}" decktap-universal
)

echo "==> Successfully created:"
echo "    App: ${APP_BUNDLE}"
echo "    DMG: ${DIST_DIR}/${DMG_NAME} ($(du -sh "${DIST_DIR}/${DMG_NAME}" | cut -f1))"
echo "    Tar: ${DIST_DIR}/${TAR_NAME} ($(du -sh "${DIST_DIR}/${TAR_NAME}" | cut -f1))"
