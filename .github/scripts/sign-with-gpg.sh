#!/usr/bin/env bash
# Sign artifacts with GPG
set -euo pipefail

ARTIFACT_DIR="${1:?Usage: $0 <artifact-dir>}"
GPG_PRIVATE_KEY="${2:?Usage: $0 <artifact-dir> <gpg-private-key>}"
GPG_PASSPHRASE="${3:-}"

if [ ! -d "$ARTIFACT_DIR" ]; then
  echo "❌ Artifact directory not found: $ARTIFACT_DIR"
  exit 1
fi

echo "🔐 Setting up GPG..."

# Import GPG private key
echo "$GPG_PRIVATE_KEY" | gpg --batch --import

# Get key ID
KEY_ID=$(echo "$GPG_PRIVATE_KEY" | gpg --list-packets 2>/dev/null | grep 'keyid:' | head -1 | awk '{print $NF}')
echo "🔑 Using GPG Key ID: $KEY_ID"

echo ""
echo "📝 Signing artifacts with GPG..."

# Create temp file for tracking (avoids subshell counter issues)
TEMP_STATS=$(mktemp)
echo "0 0" > "$TEMP_STATS" # signed failed

# Sign all tarballs
while IFS= read -r artifact; do
  echo "📝 Signing: $(basename "$artifact")"

  # Check if already signed
  if [ -f "${artifact}.asc" ]; then
    echo "⏭️  Already signed: $(basename "$artifact")"
    continue
  fi

  if [ -n "$GPG_PASSPHRASE" ]; then
    echo "$GPG_PASSPHRASE" | gpg --batch --yes --passphrase-fd 0 \
      --detach-sign --armor \
      --output "${artifact}.asc" \
      "$artifact" 2>/dev/null
  else
    gpg --batch --yes \
      --detach-sign --armor \
      --output "${artifact}.asc" \
      "$artifact" 2>/dev/null
  fi

  if [ $? -eq 0 ]; then
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

  if [ -f "${checksum}.asc" ]; then
    echo "⏭️  Already signed: $(basename "$checksum")"
    continue
  fi

  if [ -n "$GPG_PASSPHRASE" ]; then
    echo "$GPG_PASSPHRASE" | gpg --batch --yes --passphrase-fd 0 \
      --detach-sign --armor \
      --output "${checksum}.asc" \
      "$checksum" 2>/dev/null
  else
    gpg --batch --yes \
      --detach-sign --armor \
      --output "${checksum}.asc" \
      "$checksum" 2>/dev/null
  fi

  if [ $? -eq 0 ]; then
    echo "✅ Signed checksum: $(basename "$checksum")"
    read -r signed failed < "$TEMP_STATS"
    echo "$((signed + 1)) $failed" > "$TEMP_STATS"
  else
    echo "❌ Failed to sign checksum: $(basename "$checksum")"
    read -r signed failed < "$TEMP_STATS"
    echo "$signed $((failed + 1))" > "$TEMP_STATS"
  fi
done < <(find "$ARTIFACT_DIR" -name '*.sha256' -type f 2>/dev/null)

# Sign SBOM files
while IFS= read -r sbom; do
  echo "📝 Signing SBOM: $(basename "$sbom")"

  if [ -f "${sbom}.asc" ]; then
    echo "⏭️  Already signed: $(basename "$sbom")"
    continue
  fi

  if [ -n "$GPG_PASSPHRASE" ]; then
    echo "$GPG_PASSPHRASE" | gpg --batch --yes --passphrase-fd 0 \
      --detach-sign --armor \
      --output "${sbom}.asc" \
      "$sbom" 2>/dev/null
  else
    gpg --batch --yes \
      --detach-sign --armor \
      --output "${sbom}.asc" \
      "$sbom" 2>/dev/null
  fi

  if [ $? -eq 0 ]; then
    echo "✅ Signed SBOM: $(basename "$sbom")"
    read -r signed failed < "$TEMP_STATS"
    echo "$((signed + 1)) $failed" > "$TEMP_STATS"
  else
    echo "❌ Failed to sign SBOM: $(basename "$sbom")"
    read -r signed failed < "$TEMP_STATS"
    echo "$signed $((failed + 1))" > "$TEMP_STATS"
  fi
done < <(find "$ARTIFACT_DIR" -name '*.sbom.json' -type f 2>/dev/null)

# Read final counts
read -r SIGNED_COUNT FAILED_COUNT < "$TEMP_STATS"
rm -f "$TEMP_STATS"

echo ""
echo "📊 GPG Signing Summary:"
echo "  Signed: $SIGNED_COUNT files"
echo "  Failed: $FAILED_COUNT files"

if [ "$FAILED_COUNT" -gt 0 ]; then
  echo "⚠️  Warning: Some artifacts failed to sign"
  exit 1
fi

echo "✅ All artifacts signed successfully"
