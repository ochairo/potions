#!/usr/bin/env bash
# Sign artifacts with Cosign (keyless Sigstore signing)
# This script is optimized for CI/CD batch operations
set -euo pipefail

ARTIFACT_DIR="${1:?Usage: $0 <artifact-dir>}"

if [ ! -d "$ARTIFACT_DIR" ]; then
  echo "❌ Artifact directory not found: $ARTIFACT_DIR"
  exit 1
fi

echo "🔐 Installing Cosign..."
COSIGN_VERSION="v2.4.1"
curl -sLO "https://github.com/sigstore/cosign/releases/download/${COSIGN_VERSION}/cosign-linux-amd64"
sudo install cosign-linux-amd64 /usr/local/bin/cosign
rm -f cosign-linux-amd64
cosign version

echo ""
echo "🔑 Signing artifacts with Cosign (keyless)..."
echo "   Using GitHub OIDC for keyless signing"

# Create temp file for tracking (avoids subshell counter issues)
TEMP_STATS=$(mktemp)
echo "0 0" > "$TEMP_STATS" # signed failed

# Sign all tarballs
while IFS= read -r artifact; do
  echo "📝 Signing: $(basename "$artifact")"

  # Check if already signed
  if [ -f "${artifact}.sig" ] && [ -f "${artifact}.pem" ]; then
    echo "⏭️  Already signed: $(basename "$artifact")"
    continue
  fi

  # Keyless signing with Sigstore (uses OIDC token from GitHub Actions)
  if COSIGN_EXPERIMENTAL=1 cosign sign-blob "$artifact" \
    --output-signature="${artifact}.sig" \
    --output-certificate="${artifact}.pem" \
    --yes 2>&1; then
    echo "✅ Signed: $(basename "$artifact")"
    read -r signed failed < "$TEMP_STATS"
    echo "$((signed + 1)) $failed" > "$TEMP_STATS"
  else
    echo "❌ Failed to sign: $(basename "$artifact")"
    read -r signed failed < "$TEMP_STATS"
    echo "$signed $((failed + 1))" > "$TEMP_STATS"
  fi
done < <(find "$ARTIFACT_DIR" -name '*.tar.gz' -type f 2>/dev/null)

# Sign checksums
while IFS= read -r checksum; do
  echo "📝 Signing checksum: $(basename "$checksum")"

  if [ -f "${checksum}.sig" ] && [ -f "${checksum}.pem" ]; then
    echo "⏭️  Already signed: $(basename "$checksum")"
    continue
  fi

  if COSIGN_EXPERIMENTAL=1 cosign sign-blob "$checksum" \
    --output-signature="${checksum}.sig" \
    --output-certificate="${checksum}.pem" \
    --yes 2>&1; then
    echo "✅ Signed checksum: $(basename "$checksum")"
    read -r signed failed < "$TEMP_STATS"
    echo "$((signed + 1)) $failed" > "$TEMP_STATS"
  else
    echo "❌ Failed to sign checksum: $(basename "$checksum")"
    read -r signed failed < "$TEMP_STATS"
    echo "$signed $((failed + 1))" > "$TEMP_STATS"
  fi
done < <(find "$ARTIFACT_DIR" -name '*.sha256' -type f 2>/dev/null)

# Read final counts
read -r SIGNED_COUNT FAILED_COUNT < "$TEMP_STATS"
rm -f "$TEMP_STATS"

echo ""
echo "📊 Cosign Signing Summary:"
echo "  Signed: $SIGNED_COUNT files"
echo "  Failed: $FAILED_COUNT files"

if [ "$FAILED_COUNT" -gt 0 ]; then
  echo "⚠️  Warning: Some artifacts failed to sign"
  exit 1
fi

echo "✅ All artifacts signed successfully"
