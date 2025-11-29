#!/bin/bash
# Check and cleanup disk space if needed
# Usage: check-disk-space.sh [min_gb]
# Default minimum: 20GB

set -euo pipefail

min_gb="${1:-20}"

echo "💾 Checking available disk space..."
df -h

available_gb=$(df -BG . | awk 'NR==2 {print $4}' | sed 's/G//')
echo "📊 Available space: ${available_gb}GB"

if [ "$available_gb" -lt "$min_gb" ]; then
  echo "⚠️  WARNING: Low disk space (${available_gb}GB available, need ${min_gb}GB)"
  echo "🧹 Cleaning up to free space..."

  sudo rm -rf /usr/share/dotnet
  sudo rm -rf /opt/ghc
  sudo rm -rf /usr/local/share/boost
  sudo rm -rf "$AGENT_TOOLSDIRECTORY"

  echo "✅ Cleanup complete"
  df -h
else
  echo "✅ Sufficient disk space available"
fi
