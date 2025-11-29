#!/bin/bash
# Filter packages that support the specified platform
# Usage: filter-packages-by-platform.sh <platform> <packages_json>
# Output: Filtered JSON array of packages

set -euo pipefail

platform="$1"
packages_json="$2"

# Map platform names to acceptable variants
case "$platform" in
  "linux-x86_64")
    filter_expr='[.[] | select(.platforms | any(. == "linux-x86_64" or . == "linux-amd64"))]'
    ;;
  "linux-arm64")
    filter_expr='[.[] | select(.platforms | any(. == "linux-arm64"))]'
    ;;
  "macos-x86_64"|"darwin-x86_64")
    filter_expr='[.[] | select(.platforms | any(. == "macos-x86_64" or . == "darwin-amd64"))]'
    ;;
  "macos-arm64"|"darwin-arm64")
    filter_expr='[.[] | select(.platforms | any(. == "macos-arm64" or . == "darwin-arm64"))]'
    ;;
  *)
    echo "❌ Unknown platform: $platform" >&2
    exit 1
    ;;
esac

# Filter and output
filtered=$(echo "$packages_json" | jq -c "$filter_expr")
filtered_count=$(echo "$filtered" | jq 'length')
total_count=$(echo "$packages_json" | jq 'length')

if [ "$filtered_count" -lt "$total_count" ]; then
  skipped=$((total_count - filtered_count))
  echo "⏭️  Skipping $skipped packages that don't support $platform" >&2
fi

echo "📦 Building $filtered_count/$total_count packages for $platform" >&2
echo "$filtered"
