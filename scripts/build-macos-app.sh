#!/bin/bash
# Build macOS .app bundle for AgentManager Helper
set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Configuration
APP_NAME="AgentManager Helper"
APP_BUNDLE="${APP_NAME}.app"
BINARY_NAME="agentmgr-helper"
CLI_NAME="agentmgr"
BUNDLE_ID="net.kevinelliott.agentmgr-helper"

# Directories
BUILD_DIR="${PROJECT_ROOT}/bin"
BUNDLE_DIR="${BUILD_DIR}/${APP_BUNDLE}"
CONTENTS_DIR="${BUNDLE_DIR}/Contents"
MACOS_DIR="${CONTENTS_DIR}/MacOS"
RESOURCES_DIR="${CONTENTS_DIR}/Resources"

# Version info (passed as arguments or defaults)
VERSION="${1:-dev}"
COMMIT="${2:-none}"
DATE="${3:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

echo "Building ${APP_NAME} v${VERSION}..."

# Build the binary first if it doesn't exist
if [ ! -f "${BUILD_DIR}/${BINARY_NAME}" ]; then
    echo "Building ${BINARY_NAME} binary..."
    cd "${PROJECT_ROOT}"
    go build -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
        -o "${BUILD_DIR}/${BINARY_NAME}" ./cmd/agentmgr-helper
fi

# Clean existing bundle
rm -rf "${BUNDLE_DIR}"

# Create bundle structure
echo "Creating app bundle structure..."
mkdir -p "${MACOS_DIR}"
mkdir -p "${RESOURCES_DIR}"

# Copy binary
echo "Copying binary..."
cp "${BUILD_DIR}/${BINARY_NAME}" "${MACOS_DIR}/"

# Bundle the CLI next to the helper. Settings > Install looks for it there
# (findBundledAgentMgr), so without it the Install button always fails.
if [ ! -f "${BUILD_DIR}/${CLI_NAME}" ]; then
    echo "Building ${CLI_NAME} binary..."
    cd "${PROJECT_ROOT}"
    go build -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
        -o "${BUILD_DIR}/${CLI_NAME}" ./cmd/agentmgr
fi
echo "Copying CLI..."
cp "${BUILD_DIR}/${CLI_NAME}" "${MACOS_DIR}/"

# Create Info.plist with version info
echo "Creating Info.plist..."
cat > "${CONTENTS_DIR}/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleDevelopmentRegion</key>
    <string>en</string>
    <key>CFBundleExecutable</key>
    <string>${BINARY_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>${BUNDLE_ID}</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundleVersion</key>
    <string>${COMMIT}</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.13</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSSupportsAutomaticGraphicsSwitch</key>
    <true/>
</dict>
</plist>
EOF

# Copy icon if it exists
if [ -f "${PROJECT_ROOT}/resources/macos/AppIcon.icns" ]; then
    echo "Copying app icon..."
    cp "${PROJECT_ROOT}/resources/macos/AppIcon.icns" "${RESOURCES_DIR}/"
    # Add icon reference to Info.plist
    /usr/libexec/PlistBuddy -c "Add :CFBundleIconFile string AppIcon" "${CONTENTS_DIR}/Info.plist" 2>/dev/null || true
fi

# Create PkgInfo
echo "APPL????" > "${CONTENTS_DIR}/PkgInfo"

echo "App bundle created: ${BUNDLE_DIR}"
echo ""
echo "To run: open '${BUNDLE_DIR}'"
echo "To install: cp -r '${BUNDLE_DIR}' /Applications/"
