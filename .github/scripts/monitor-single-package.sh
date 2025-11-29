#!/bin/bash
# Monitor a single package and build package info if update needed
# Usage: monitor-single-package.sh <package_name>
# Output: JSON object with package info if update needed, empty string otherwise

set -euo pipefail

package="$1"

# Get version - use input version if provided, otherwise get latest
if [ -n "${VERSION_INPUT:-}" ]; then
  version="$VERSION_INPUT"
  echo "📦 Manual run: building $package v$version (user-specified)" >&2
else
  export GITHUB_TOKEN="${GITHUB_TOKEN:-}"
  version=$(./bin/potions monitor --json=true "$package" | jq -r '.[0].latest_version')
  unset GITHUB_TOKEN

  if [ -z "$version" ] || [ "$version" = "null" ]; then
    echo "❌ Error: Could not determine version for '$package'" >&2
    exit 1
  fi
fi

# Get normalized platforms
platforms=$(.github/scripts/normalize-platforms.sh "recipes/${package}.yml")

# Output package info as compact JSON (single line)
jq -nc \
  --arg pkg "$package" \
  --arg ver "$version" \
  --argjson plat "$platforms" \
  '{package: $pkg, version: $ver, platforms: $plat}'
