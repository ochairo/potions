#!/usr/bin/env bash
# Generate GitHub Attestations for artifacts
set -euo pipefail

ARTIFACT_DIR="${1:?Usage: $0 <artifact-dir>}"

if [ ! -d "$ARTIFACT_DIR" ]; then
  echo "❌ Artifact directory not found: $ARTIFACT_DIR"
  exit 1
fi

echo "🔐 Generating GitHub Attestations..."

ATTESTED_COUNT=0

# Note: This script prepares the artifact list for GitHub's attest-build-provenance action
# The actual attestation is done by the action in the workflow

# Create a list of all artifacts to attest
ARTIFACTS_FILE="${ARTIFACT_DIR}/artifacts-to-attest.txt"
true > "$ARTIFACTS_FILE"

# Find all tarballs
find "$ARTIFACT_DIR" -name '*.tar.gz' -type f 2>/dev/null | while read -r artifact; do
  echo "📝 Preparing attestation for: $(basename "$artifact")"
  echo "$artifact" >> "$ARTIFACTS_FILE"
  ATTESTED_COUNT=$((ATTESTED_COUNT + 1))
done

# Find all checksums
find "$ARTIFACT_DIR" -name '*.sha256' -type f 2>/dev/null | while read -r checksum; do
  echo "📝 Preparing attestation for: $(basename "$checksum")"
  echo "$checksum" >> "$ARTIFACTS_FILE"
  ATTESTED_COUNT=$((ATTESTED_COUNT + 1))
done

# Find all SBOMs
find "$ARTIFACT_DIR" -name '*.sbom.json' -type f 2>/dev/null | while read -r sbom; do
  echo "📝 Preparing attestation for: $(basename "$sbom")"
  echo "$sbom" >> "$ARTIFACTS_FILE"
  ATTESTED_COUNT=$((ATTESTED_COUNT + 1))
done

if [ ! -s "$ARTIFACTS_FILE" ]; then
  echo "⚠️  No artifacts found to attest"
  exit 1
fi

ARTIFACT_COUNT=$(wc -l < "$ARTIFACTS_FILE" | tr -d ' ')
echo ""
echo "📊 Attestation Preparation Summary:"
echo "  Artifacts prepared: $ARTIFACT_COUNT"
echo "  List saved to: $ARTIFACTS_FILE"
echo ""
echo "✅ Artifacts ready for attestation"
echo "   (Attestation will be performed by GitHub Actions)"
