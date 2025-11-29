#!/bin/bash
# Create failure tracking JSON from build failure files
# Usage: create-failure-tracking.sh <platform> <batch_id>
# Output: failure-tracking.json file

set -euo pipefail

platform="$1"
batch_id="$2"

failed_packages="[]"

# Aggregate failures from all failure files
for file in build-failures.txt build-failures-timeout.txt build-failures-error.txt; do
  if [ -f "$file" ] && [ -s "$file" ]; then
    failed=$(cat "$file" | sed 's/^ *//' | jq -R . | jq -s -c .)
    failed_packages=$(echo "$failed_packages" | jq -c ". += $failed")
  fi
done

# Deduplicate and sort
failed_packages=$(echo "$failed_packages" | jq -c 'unique | sort')

# Create tracking file
cat > failure-tracking.json <<EOF
{
  "platform": "$platform",
  "batch_id": "$batch_id",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "failed_packages": $failed_packages
}
EOF

cat failure-tracking.json
