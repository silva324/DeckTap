#!/usr/bin/env bash
# update-homebrew-cask.sh — Updates Casks/decktap.rb in silva324/homebrew-tap
set -euo pipefail

REPO="silva324/DeckTap"
TAP_REPO="silva324/homebrew-tap"

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  VERSION=$(gh release view --repo "$REPO" --json tagName --jq '.tagName' | sed 's/^v//')
fi
VERSION="${VERSION#v}"

echo "==> Fetching SHA256 for DeckTap v${VERSION} macOS Universal DMG..."
DMG_URL="https://github.com/${REPO}/releases/download/v${VERSION}/DeckTap-v${VERSION}-macOS-Universal.dmg"
SHA256=$(curl -sL "$DMG_URL" | shasum -a 256 | awk '{print $1}')

if [ -z "$SHA256" ] || [ "${#SHA256}" -ne 64 ]; then
  echo "Error: Failed to compute valid SHA256 checksum from $DMG_URL"
  exit 1
fi

echo "    Version: ${VERSION}"
echo "    SHA256:  ${SHA256}"

CASK_CONTENT=$(cat <<EOF
cask "decktap" do
  version "${VERSION}"
  sha256 "${SHA256}"

  url "https://github.com/silva324/DeckTap/releases/download/v#{version}/DeckTap-v#{version}-macOS-Universal.dmg"
  name "DeckTap"
  desc "Control presentations from your phone with notes and laser pointer"
  homepage "https://github.com/silva324/DeckTap"

  livecheck do
    url :url
    strategy :github_latest
  end

  auto_updates true
  depends_on :macos

  app "DeckTap.app"

  zap trash: [
    "~/.decktap",
    "~/Library/Application Support/DeckTap",
    "~/Library/Preferences/com.decktap.plist",
  ]
end
EOF
)

ENCODED_CONTENT=$(echo "$CASK_CONTENT" | base64)

FILE_SHA=$(gh api "repos/${TAP_REPO}/contents/Casks/decktap.rb" --jq '.sha' 2>/dev/null || echo "")

if [ -n "$FILE_SHA" ]; then
  echo "==> Updating Casks/decktap.rb in ${TAP_REPO}..."
  gh api --method PUT "repos/${TAP_REPO}/contents/Casks/decktap.rb" \
    -f message="chore: bump decktap cask to v${VERSION}" \
    -f content="$ENCODED_CONTENT" \
    -f sha="$FILE_SHA" \
    -f branch="main" >/dev/null
else
  echo "==> Creating Casks/decktap.rb in ${TAP_REPO}..."
  gh api --method PUT "repos/${TAP_REPO}/contents/Casks/decktap.rb" \
    -f message="chore: create decktap cask v${VERSION}" \
    -f content="$ENCODED_CONTENT" \
    -f branch="main" >/dev/null
fi

echo "==> Successfully updated ${TAP_REPO}/Casks/decktap.rb to v${VERSION}!"
