#!/bin/bash
# Generate GitHub Actions workflow summary
# Usage: generate-workflow-summary.sh <run_id> <server_url> <repository> <ref_name> <detect_result> <linux_x86_result> <linux_arm_result> <macos_arm_result> <macos_x86_result> <release_result>

set -euo pipefail

run_id="$1"
server_url="$2"
repository="$3"
ref_name="$4"
detect_result="$5"
linux_x86_result="$6"
linux_arm_result="$7"
macos_arm_result="$8"
macos_x86_result="$9"
release_result="${10}"

# Helper function to convert result to emoji
result_to_emoji() {
  case "$1" in
    success) echo "✅ Success" ;;
    failure) echo "❌ Failed" ;;
    cancelled) echo "⏹️ Cancelled" ;;
    *) echo "⏭️ Skipped" ;;
  esac
}

# Generate header
{
  echo "## 📊 Scheduled Release Workflow Summary"
  echo ""
  echo "**Workflow Run:** [${run_id}](${server_url}/${repository}/actions/runs/${run_id})"
  echo "**Trigger:** ${GITHUB_EVENT_NAME:-manual}"
  echo "**Branch:** ${ref_name}"
  echo ""

  # Job status table
  echo "### Job Status"
  echo ""
  echo "| Job | Status |"
  echo "|-----|--------|"
  echo "| Detect Updates | $(result_to_emoji "$detect_result") |"
  echo "| Build Linux x86_64 | $(result_to_emoji "$linux_x86_result") |"
  echo "| Build Linux ARM64 | $(result_to_emoji "$linux_arm_result") |"
  echo "| Build macOS ARM64 | $(result_to_emoji "$macos_arm_result") |"
  echo "| Build macOS Intel | $(result_to_emoji "$macos_x86_result") |"
  echo "| Release Packages | $(result_to_emoji "$release_result") |"
  echo ""
} >> "$GITHUB_STEP_SUMMARY"

# Process success reports
if [ -d success-reports ]; then
  echo "### ✅ Successfully Built Packages by Platform" >> "$GITHUB_STEP_SUMMARY"
  echo "" >> "$GITHUB_STEP_SUMMARY"

  # Merge and deduplicate
  cat success-reports/*.txt 2>/dev/null | sort -u > all-successes.txt || touch all-successes.txt

  if [ -s all-successes.txt ]; then
    success_count=$(wc -l < all-successes.txt | tr -d ' ')
    echo "**Total: $success_count unique package versions**" >> "$GITHUB_STEP_SUMMARY"
    echo "" >> "$GITHUB_STEP_SUMMARY"

    echo "<details>" >> "$GITHUB_STEP_SUMMARY"
    echo "<summary>View by package</summary>" >> "$GITHUB_STEP_SUMMARY"
    echo "" >> "$GITHUB_STEP_SUMMARY"

    while IFS=: read -r package version; do
      echo "#### $package v$version" >> "$GITHUB_STEP_SUMMARY"
      echo "" >> "$GITHUB_STEP_SUMMARY"
      echo "**Platforms:**" >> "$GITHUB_STEP_SUMMARY"

      # Check all platforms at once
      for platform_file in success-reports/*.txt; do
        if grep -Fxq "${package}:${version}" "$platform_file" 2>/dev/null; then
          platform=$(basename "$platform_file" | sed 's/-batch-.*$//')
          echo "- ✅ $platform" >> "$GITHUB_STEP_SUMMARY"
        fi
      done
      echo "" >> "$GITHUB_STEP_SUMMARY"
    done < all-successes.txt

    echo "</details>" >> "$GITHUB_STEP_SUMMARY"
    echo "" >> "$GITHUB_STEP_SUMMARY"
  else
    echo "*No successful builds*" >> "$GITHUB_STEP_SUMMARY"
    echo "" >> "$GITHUB_STEP_SUMMARY"
  fi
fi

# Process failure reports
if [ -d failure-reports ]; then
  cat failure-reports/*-failures.txt 2>/dev/null | sort -u > all-failures.txt || touch all-failures.txt

  if [ -s all-failures.txt ]; then
    failure_count=$(wc -l < all-failures.txt | tr -d ' ')
    echo "### ❌ Failed Builds by Platform" >> "$GITHUB_STEP_SUMMARY"
    echo "" >> "$GITHUB_STEP_SUMMARY"
    echo "**Total: $failure_count failed builds**" >> "$GITHUB_STEP_SUMMARY"
    echo "" >> "$GITHUB_STEP_SUMMARY"

    echo "<details>" >> "$GITHUB_STEP_SUMMARY"
    echo "<summary>View failures by platform</summary>" >> "$GITHUB_STEP_SUMMARY"
    echo "" >> "$GITHUB_STEP_SUMMARY"

    for platform in linux-x86_64 linux-arm64 macos-arm64 macos-x86_64; do
      # Collect platform-specific failures
      cat failure-reports/${platform}-*-failures*.txt 2>/dev/null | sort -u > platform-failures.txt || touch platform-failures.txt

      if [ -s platform-failures.txt ]; then
        platform_count=$(wc -l < platform-failures.txt | tr -d ' ')
        platform_name=$(echo "$platform" | sed 's/linux-x86_64/Linux x86_64/; s/linux-arm64/Linux ARM64/; s/macos-arm64/macOS ARM64/; s/macos-x86_64/macOS Intel/')

        echo "#### $platform_name ($platform_count failures)" >> "$GITHUB_STEP_SUMMARY"
        echo "" >> "$GITHUB_STEP_SUMMARY"

        while read -r line; do
          if echo "$line" | grep -q "TIMEOUT"; then
            echo "- ⏱️ $line" >> "$GITHUB_STEP_SUMMARY"
          else
            echo "- ❌ $line" >> "$GITHUB_STEP_SUMMARY"
          fi
        done < platform-failures.txt
        echo "" >> "$GITHUB_STEP_SUMMARY"
      fi
    done

    echo "</details>" >> "$GITHUB_STEP_SUMMARY"
    echo "" >> "$GITHUB_STEP_SUMMARY"

    rm -f platform-failures.txt
  fi

  rm -f all-failures.txt
fi

rm -f all-successes.txt

# Overall status
echo "### Overall Status" >> "$GITHUB_STEP_SUMMARY"
echo "" >> "$GITHUB_STEP_SUMMARY"

if [ "$detect_result" = "failure" ]; then
  echo "❌ **Workflow Failed** - Package detection failed. Check the detect-updates job logs." >> "$GITHUB_STEP_SUMMARY"
elif [ "$release_result" = "failure" ]; then
  echo "⚠️ **Partial Success** - Builds completed but release creation encountered issues. Check the release-packages job logs." >> "$GITHUB_STEP_SUMMARY"
elif [ "$linux_x86_result" = "failure" ] || [ "$linux_arm_result" = "failure" ] || [ "$macos_arm_result" = "failure" ] || [ "$macos_x86_result" = "failure" ]; then
  echo "⚠️ **Build Failures Detected** - Some builds failed. Partial releases may have been created for successful builds." >> "$GITHUB_STEP_SUMMARY"
elif [ "$release_result" = "success" ]; then
  echo "✅ **Workflow Completed Successfully** - All builds and releases completed." >> "$GITHUB_STEP_SUMMARY"
else
  echo "ℹ️ **No Updates Found** - No package updates detected. No builds or releases were created." >> "$GITHUB_STEP_SUMMARY"
fi
