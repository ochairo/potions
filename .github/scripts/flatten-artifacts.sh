#!/bin/bash
# Flatten nested artifact directory structure
# Usage: flatten-artifacts.sh <artifacts_dir>

set -euo pipefail

artifacts_dir="${1:-current-artifacts}"

echo "📁 Flattening artifacts directory..."

before_count=$(find "$artifacts_dir" -type f \( -name '*.tar.gz' -o -name '*.sha256' -o -name '*.sha512' -o -name '*.sbom.json' -o -name '*.provenance.json' \) | wc -l | tr -d ' ')
echo "📊 Found $before_count artifact files" >&2

# Create temp directory
temp_dir="${artifacts_dir}-flat"
mkdir -p "$temp_dir"

# Move all artifact files to flat structure
find "$artifacts_dir" -type f \( \
  -name '*.tar.gz' -o \
  -name '*.sha256' -o \
  -name '*.sha512' -o \
  -name '*.sbom.json' -o \
  -name '*.provenance.json' \
\) -exec mv {} "$temp_dir/" \;

# Replace original with flattened
rm -rf "$artifacts_dir"
mv "$temp_dir" "$artifacts_dir"

final_count=$(find "$artifacts_dir" -type f | wc -l | tr -d ' ')
echo "✅ Flattened $final_count files" >&2

if [ "$final_count" -eq 0 ]; then
  echo "❌ No files found after flattening!" >&2
  exit 1
fi

ls -lh "$artifacts_dir/" | head -5 >&2
