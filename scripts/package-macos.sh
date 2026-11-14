#!/bin/bash
# Package AgentManager.app for distribution: universal binaries, code
# signature, notarization and a DMG.
#
#   scripts/package-macos.sh [version] [commit] [date]
#
# Signing and notarization are driven by the environment, so the same script
# works on a developer's Mac and in CI:
#
#   MACOS_SIGN_IDENTITY   "Developer ID Application: Name (TEAMID)". When
#                         unset the app is ad-hoc signed: it runs on the Mac
#                         that built it, but Gatekeeper blocks it elsewhere.
#   APPLE_ID              Apple ID used for notarization.
#   APPLE_APP_PASSWORD    App-specific password for that Apple ID.
#   APPLE_TEAM_ID         Developer team ID.
#
# Notarization runs only when a signing identity and all three APPLE_*
# values are set. Output: dist/AgentManager-<version>-macos-universal.dmg
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "${PROJECT_ROOT}"

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${2:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
DATE="${3:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

APP="bin/AgentManager.app"
DIST="dist"
DMG="${DIST}/AgentManager-${VERSION#v}-macos-universal.dmg"
LDFLAGS="-s -w -X main.version=${VERSION#v} -X main.commit=${COMMIT} -X main.date=${DATE}"

echo "==> Building universal binaries (${VERSION})"
mkdir -p bin "${DIST}"
for target in agentmgr agentmgr-helper; do
    for arch in arm64 amd64; do
        # The helper links AppKit and WebKit through cgo; the CLI is built the
        # same way here so both halves of the bundle match.
        CGO_ENABLED=1 GOOS=darwin GOARCH="${arch}" \
            go build -trimpath -ldflags "${LDFLAGS}" -o "bin/${target}-${arch}" "./cmd/${target}"
    done
    lipo -create -output "bin/${target}" "bin/${target}-arm64" "bin/${target}-amd64"
    rm -f "bin/${target}-arm64" "bin/${target}-amd64"
done

echo "==> Assembling ${APP}"
./scripts/build-macos-app.sh "${VERSION}" "${COMMIT}" "${DATE}" >/dev/null

echo "==> Signing"
if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
    SIGN_ARGS=(--force --timestamp --options runtime --sign "${MACOS_SIGN_IDENTITY}")
else
    echo "    MACOS_SIGN_IDENTITY is not set: ad-hoc signing (not distributable)"
    SIGN_ARGS=(--force --sign -)
fi
# Inner executables first, then the bundle that seals them.
codesign "${SIGN_ARGS[@]}" "${APP}/Contents/MacOS/agentmgr"
codesign "${SIGN_ARGS[@]}" "${APP}/Contents/MacOS/agentmgr-helper"
codesign "${SIGN_ARGS[@]}" "${APP}"
codesign --verify --strict --deep "${APP}"

echo "==> Building ${DMG}"
STAGE="$(mktemp -d)"
trap 'rm -rf "${STAGE}"' EXIT
cp -R "${APP}" "${STAGE}/"
ln -s /Applications "${STAGE}/Applications"
rm -f "${DMG}"
hdiutil create -quiet -volname "AgentManager" -srcfolder "${STAGE}" -fs HFS+ -format UDZO "${DMG}"

if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
    codesign --force --timestamp --sign "${MACOS_SIGN_IDENTITY}" "${DMG}"
fi

if [ -n "${MACOS_SIGN_IDENTITY:-}" ] && [ -n "${APPLE_ID:-}" ] && [ -n "${APPLE_APP_PASSWORD:-}" ] && [ -n "${APPLE_TEAM_ID:-}" ]; then
    echo "==> Notarizing (this can take a few minutes)"
    xcrun notarytool submit "${DMG}" \
        --apple-id "${APPLE_ID}" --password "${APPLE_APP_PASSWORD}" --team-id "${APPLE_TEAM_ID}" \
        --wait
    xcrun stapler staple "${DMG}"
    spctl --assess --type open --context context:primary-signature --verbose "${DMG}"
else
    echo "==> Skipping notarization (needs MACOS_SIGN_IDENTITY, APPLE_ID, APPLE_APP_PASSWORD, APPLE_TEAM_ID)"
fi

echo "Done: ${DMG}"
