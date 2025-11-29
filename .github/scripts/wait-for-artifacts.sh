#!/bin/bash
# Wait for all build artifacts to be uploaded and stable
# Usage: wait-for-artifacts.sh <packages_json> <run_id>
# Output: Success (exit 0) or failure (exit 1)

set -euo pipefail

packages="$1"
run_id="$2"

echo "⏳ Waiting for all build artifacts to be uploaded..."

package_count=$(echo "$packages" | jq -r 'length')

# Calculate total expected artifacts
expected_total=0
for i in $(seq 0 $(( package_count - 1 ))); do
  pkg_name=$(echo "$packages" | jq -r ".[$i].package")
  pkg_platforms=$(echo "$packages" | jq -r ".[$i].platforms | length")
  echo "📦 $pkg_name: expecting $pkg_platforms platform artifacts" >&2
  expected_total=$(( expected_total + pkg_platforms ))
done

echo "📊 Expecting: $package_count packages, $expected_total artifacts" >&2

max_wait=120
poll_interval=60
stable_required=2
elapsed=0
stable_count=0
last_count=0

while [ $elapsed -lt $max_wait ]; do
  current_count=$(gh api repos/"${GITHUB_REPOSITORY}"/actions/runs/"${run_id}"/artifacts \
    --jq '.artifacts[] | select(.name | endswith("-builds")) | .name' 2>/dev/null | wc -l | tr -d ' ')

  echo "[$elapsed s] Found $current_count/$expected_total artifact(s)" >&2

  # Check stability
  if [ "$current_count" -eq "$last_count" ]; then
    stable_count=$((stable_count + 1))
  else
    stable_count=0
    last_count=$current_count
  fi

  # Exit if all artifacts present and stable
  if [ "$current_count" -eq "$expected_total" ] && [ "$stable_count" -ge "$stable_required" ]; then
    echo "✅ All $expected_total artifacts present and stable" >&2
    exit 0
  fi

  echo "  Stability: $stable_count/$stable_required (waiting ${poll_interval}s...)" >&2
  sleep $poll_interval
  elapsed=$((elapsed + poll_interval))
done

if [ "$current_count" -eq 0 ]; then
  echo "❌ No artifacts found after ${max_wait}s" >&2
  exit 1
elif [ "$current_count" -lt "$expected_total" ]; then
  echo "❌ Missing $(( expected_total - current_count )) artifacts after timeout" >&2
  exit 1
fi
