#!/bin/bash
# Download all build artifacts for a workflow run
# Usage: download-artifacts.sh <run_id> <output_dir>

set -euo pipefail

run_id="$1"
output_dir="${2:-current-artifacts}"

echo "📥 Downloading artifacts from run $run_id"
rm -rf "$output_dir"
mkdir -p "$output_dir"

# Get artifact list
echo "🔍 Fetching artifact list..."
gh api repos/"${GITHUB_REPOSITORY}"/actions/runs/"${run_id}"/artifacts \
  --jq '.artifacts[] | select(.name | endswith("-builds")) | .name' \
  > artifact-list.txt || touch artifact-list.txt

artifact_count=$(wc -l < artifact-list.txt | tr -d ' ')
echo "📦 Found $artifact_count build artifact(s)" >&2

if [ "$artifact_count" -eq 0 ]; then
  echo "❌ No artifacts found!" >&2
  exit 1
fi

# Show breakdown
echo "Artifact breakdown:" >&2
for platform in "linux-x86_64" "linux-arm64" "macos-arm64" "macos-x86_64"; do
  count=$(grep -c "$platform" artifact-list.txt 2>/dev/null || echo "0")
  echo "  $platform: $count artifacts" >&2
done

# Download artifacts
downloaded=0
failed=0

while read -r artifact_name; do
  [ -z "$artifact_name" ] && continue

  echo "  📥 Downloading: $artifact_name" >&2

  set +e
  timeout 300 gh run download "$run_id" \
    --repo "${GITHUB_REPOSITORY}" \
    --name "$artifact_name" \
    --dir "$output_dir/$artifact_name" 2>&1 | tee download-output.log >&2
  exit_code=$?
  set -e

  if [ $exit_code -eq 0 ]; then
    echo "  ✅ Downloaded: $artifact_name" >&2
    downloaded=$((downloaded + 1))
  else
    echo "  ❌ FAILED: $artifact_name" >&2
    failed=$((failed + 1))
  fi

  rm -f download-output.log
done < artifact-list.txt

echo "" >&2
echo "📊 Downloaded: $downloaded artifacts, Failed: $failed" >&2

if [ "$downloaded" -eq 0 ]; then
  echo "❌ No artifacts downloaded successfully" >&2
  exit 1
fi

file_count=$(find "$output_dir" -name '*.tar.gz' -type f 2>/dev/null | wc -l | tr -d ' ')
total_size=$(du -sh "$output_dir" 2>/dev/null | cut -f1)
echo "📊 Downloaded $file_count package files, Total: $total_size" >&2

rm -f artifact-list.txt
