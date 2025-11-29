#!/bin/bash
# Get recently failed packages from workflow runs (last 3 days)
# Usage: get-failed-packages.sh
# Output: JSON array of failed package names

set -euo pipefail

cutoff_date=$(date -u -d "3 days ago" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v-3d +%Y-%m-%dT%H:%M:%SZ)

failed_run_ids=$(gh run list \
  --workflow "scheduled-release.yml" \
  --limit 100 \
  --json conclusion,databaseId,createdAt \
  --jq "[.[] | select(.createdAt > \"$cutoff_date\" and (.conclusion == \"failure\" or .conclusion == \"timed_out\")) | .databaseId] | join(\" \")")

if [ -z "$failed_run_ids" ]; then
  echo "[]"
  exit 0
fi

# Download and parse failure artifacts
for run_id in $failed_run_ids; do
  gh run download "$run_id" --pattern "*-failures*.txt" --dir "temp-$run_id" 2>/dev/null || continue
  find "temp-$run_id" -name "*.txt" -exec cat {} \; 2>/dev/null
  rm -rf "temp-$run_id"
done | awk '{print $1}' | sort -u | jq -R . | jq -s -c .
