#!/bin/bash
# Normalize platform names (darwin->macos, amd64->x86_64)
# Usage: normalize_platforms "recipes/curl.yml"
# Output: JSON array of normalized platforms

set -euo pipefail

recipe_file="$1"
platforms
platforms=$(yq eval '.download.platforms | keys' "$recipe_file" 2>/dev/null | grep -v '^$' | sed 's/^- //' | tr '\n' ' ' || echo "")

normalized="[]"
for platform in $platforms; do
  norm
  norm=$(echo "$platform" | sed 's/darwin/macos/g' | sed 's/amd64/x86_64/g')
  normalized=$(echo "$normalized" | jq -c ". += [\"$norm\"]")
done

normalized=$(echo "$normalized" | jq -c 'unique')

# Default to all 4 platforms if none found
if [ "$normalized" = "[]" ]; then
  normalized='["linux-x86_64","linux-arm64","macos-x86_64","macos-arm64"]'
fi

echo "$normalized"
