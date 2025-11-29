#!/bin/bash
# Monitor all packages and find updates
# Usage: monitor-all-packages.sh <max_updates> <failed_packages_json>
# Output: JSON array of packages with updates

set -euo pipefail

max_updates="${1:-1}"
failed_packages="${2:-[]}"

# Get all package names from recipes/*.yml files
monitored_packages=$(find recipes -maxdepth 1 -name '*.yml' -type f -print0 2>/dev/null | xargs -0 -n1 basename | sed 's/\.yml$//' | tr '\n' ' ' | sed 's/ $//' || echo "")

if [ -z "$monitored_packages" ]; then
  echo "⚠️  Warning: No package definitions found in recipes/" >&2
  echo "[]"
  exit 0
fi

echo "📦 Found packages: $monitored_packages" >&2

found_packages="[]"
updates_found=0
packages_checked=0
packages_skipped=0
total_packages=$(echo "$monitored_packages" | wc -w)

echo "📋 Starting package update check..." >&2
echo "   Max updates to build: $max_updates" >&2
echo "   Failed packages to skip: $(echo "$failed_packages" | jq 'length')" >&2
echo "" >&2

for pkg in $monitored_packages; do
  # Skip recently failed packages
  if echo "$failed_packages" | jq -e --arg pkg "$pkg" 'any(. == $pkg)' > /dev/null 2>&1; then
    packages_skipped=$(( packages_skipped + 1 ))
    echo "⏭️  [$packages_skipped skipped] Skipping $pkg (recently failed)" >&2
    continue
  fi

  packages_checked=$(( packages_checked + 1 ))
  processed=$(( packages_checked + packages_skipped ))
  echo -n "[$processed/$total_packages] Checking $pkg... " >&2

  # Monitor this single package
  set +e
  result=$(timeout 30 ./bin/potions monitor --json=true "$pkg" 2>&1)
  monitor_exit=$?
  set -e

  if [ $monitor_exit -eq 124 ]; then
    echo "⏱️  TIMEOUT" >&2
    continue
  fi

  if [ $monitor_exit -ne 0 ] || ! echo "$result" | jq empty 2>/dev/null; then
    echo "❌ ERROR" >&2
    continue
  fi

  # Check if this package needs update
  needs_update=$(echo "$result" | jq -r '.[0].update_needed // false')
  if [ "$needs_update" = "true" ]; then
    version=$(echo "$result" | jq -r '.[0].latest_version')
    platforms=$(.github/scripts/normalize-platforms.sh "recipes/${pkg}.yml")

    # Only add to build queue if we haven't reached the limit
    if [ $updates_found -lt "$max_updates" ]; then
      echo "✅ UPDATE: v$version (will build)" >&2
      found_packages=$(echo "$found_packages" | jq -c ". += [{\"package\": \"$pkg\", \"version\": \"$version\", \"platforms\": $platforms}]")
      updates_found=$(( updates_found + 1 ))

      # Stop checking once we've found enough updates to build
      if [ $updates_found -ge "$max_updates" ]; then
        echo "✋ Reached max updates limit ($max_updates), stopping scan" >&2
        break
      fi
    else
      echo "✅ UPDATE: v$version (queued for next run)" >&2
    fi
  else
    echo "✓ UP TO DATE" >&2
  fi
done

echo "" >&2
echo "📊 Summary: $updates_found updates found, $packages_skipped skipped, $packages_checked checked" >&2

echo "$found_packages"
