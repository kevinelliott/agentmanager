#!/usr/bin/env bash
# Stores an App Store Connect API key as the GitHub secrets the "macOS app"
# workflow uses to notarize AgentManager.app.
#
# Create the key at https://appstoreconnect.apple.com/access/integrations/api
# (Team Keys, role "Developer"), download the .p8 file once, then run:
#
#   scripts/set-notarization-secrets.sh ~/Downloads/AuthKey_ABC123DEFG.p8 <issuer-id>
#
# The Key ID is read from the file name. The Issuer ID is shown above the
# list of keys on that page.
set -euo pipefail

KEY_FILE="${1:-}"
ISSUER_ID="${2:-}"
REPO="${REPO:-kevinelliott/agentmanager}"

if [ -z "${KEY_FILE}" ] || [ -z "${ISSUER_ID}" ]; then
    echo "usage: $0 <path to AuthKey_KEYID.p8> <issuer id>" >&2
    exit 2
fi
if ! grep -q "BEGIN PRIVATE KEY" "${KEY_FILE}"; then
    echo "error: ${KEY_FILE} does not look like an App Store Connect .p8 key" >&2
    exit 1
fi

KEY_ID="$(basename "${KEY_FILE}" .p8)"
KEY_ID="${KEY_ID#AuthKey_}"

base64 < "${KEY_FILE}" | tr -d '\n' | gh secret set APPLE_API_KEY_P8 --repo "${REPO}"
gh secret set APPLE_API_KEY_ID --repo "${REPO}" --body "${KEY_ID}"
gh secret set APPLE_API_ISSUER_ID --repo "${REPO}" --body "${ISSUER_ID}"

echo "Set APPLE_API_KEY_P8, APPLE_API_KEY_ID (${KEY_ID}) and APPLE_API_ISSUER_ID on ${REPO}."
