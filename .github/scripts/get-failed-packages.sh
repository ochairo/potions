#!/bin/bash
# Get recently failed packages from workflow runs (last 1 day)
# Usage: get-failed-packages.sh
# Output: JSON array of failed package names

set -euo pipefail

cutoff_date=$(date -u -d "1 day ago" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v-1d +%Y-%m-%dT%H:%M:%SZ)

echo "🔍 Checking recent failures (last 1 day since $cutoff_date)..." >&2

failed_run_ids=$(gh run list \
  --workflow "scheduled-release.yml" \
  --limit 100 \
  --json conclusion,databaseId,createdAt \
  --jq "[.[] | select(.createdAt > \"$cutoff_date\" and (.conclusion == \"failure\" or .conclusion == \"timed_out\")) | .databaseId] | join(\" \")")

if [ -z "$failed_run_ids" ]; then
  echo "✅ No recent failed runs found" >&2
  echo "[]"
  exit 0
fi

run_count=$(echo "$failed_run_ids" | wc -w | tr -d ' ')
echo "📥 Found $run_count failed runs, downloading artifacts..." >&2

# Download and parse failure artifacts
# Pattern: linux-x86_64-batch-0-failures, macos-arm64-batch-1-failures
processed_runs=0

for run_id in $failed_run_ids; do
  echo "  Checking run $run_id..." >&2

  # Download failure artifacts (pattern without .txt extension)
  if gh run download "$run_id" --pattern "*-failures" --dir "temp-$run_id" 2>/dev/null; then
    # Parse all failure files: extract package name from "package v1.0.0 (platform)" format
    artifacts=$(find "temp-$run_id" -name "*.txt" -type f 2>/dev/null)

    if [ -n "$artifacts" ]; then
      # Extract package names from lines like: "curl v8.11.1 (linux-x86_64) - TIMEOUT"
      while IFS= read -r artifact_file; do
        if [ -s "$artifact_file" ]; then
          # Extract just the package name (first field before space and 'v')
          awk '{print $1}' "$artifact_file" | grep -v '^$'
        fi
      done <<< "$artifacts"
    fi

    processed_runs=$((processed_runs + 1))
  fi

  rm -rf "temp-$run_id"
done > /tmp/all-failures.txt

# Deduplicate and convert to JSON array
if [ -s /tmp/all-failures.txt ]; then
  failed_count=$(sort -u /tmp/all-failures.txt | wc -l | tr -d ' ')
  echo "⚠️  Found $failed_count unique failed packages from $processed_runs runs" >&2

  sort -u /tmp/all-failures.txt | jq -R . | jq -s -c .
else
  echo "✅ No failure artifacts found" >&2
  echo "[]"
fi

rm -f /tmp/all-failures.txt
