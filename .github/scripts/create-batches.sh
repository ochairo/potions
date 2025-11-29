#!/bin/bash
# Create batches from packages JSON
# Usage: create-batches.sh <packages_json>
# Output: JSON array of batches

set -euo pipefail

packages="$1"

# Validate input
if [ -z "$packages" ] || [ "$packages" = "null" ] || [ "$packages" = "[]" ]; then
  echo "[]"
  exit 0
fi

if ! echo "$packages" | jq empty 2>/dev/null; then
  echo "❌ Invalid JSON" >&2
  exit 1
fi

package_count=$(echo "$packages" | jq -r 'length')
echo "📊 Creating $package_count batches ($(( package_count * 4 )) total platform jobs)" >&2

# Create one batch per package
batches="[]"
for i in $(seq 0 $(( package_count - 1 ))); do
  package=$(echo "$packages" | jq -c ".[$i:$(( i + 1 ))]")
  batches=$(echo "$batches" | jq -c ". += [{\"id\": $i, \"packages\": $package}]")
done

echo "$batches"
