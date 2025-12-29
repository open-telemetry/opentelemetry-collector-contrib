#!/bin/bash
# Script to prepare and commit StarRocks exporter

set -e

echo "🚀 Preparing StarRocks Exporter for commit..."

# Check if we're in the right directory
if [ ! -f "go.mod" ] || [ ! -d "exporter/starrocksexporter" ]; then
    echo "❌ Error: Please run this script from the repo root"
    exit 1
fi

# Create branch
BRANCH_NAME="add-starrocks-exporter"
echo "📦 Creating branch: $BRANCH_NAME"
git checkout -b "$BRANCH_NAME" 2>/dev/null || git checkout "$BRANCH_NAME"

# Stage files
echo "📝 Staging files..."
git add exporter/starrocksexporter/
git add cmd/otelcontribcol/builder-config.yaml

# Show status
echo ""
echo "📊 Files to be committed:"
git status --short

# Commit
echo ""
echo "💾 Committing..."
git commit -m "exporter/starrocks: Add StarRocks exporter

Add new exporter for StarRocks database using MySQL protocol.
Supports logs, traces, and metrics with automatic schema creation.

Features:
- Export logs, traces, and metrics to StarRocks
- MySQL protocol support (port 9030)
- Automatic database and table creation
- Connection pool configuration
- Configurable table names
- Comprehensive unit tests

Tests:
- All unit tests pass
- Config validation tests
- Factory creation tests
- Component lifecycle tests
- YAML config loading tests"

echo ""
echo "✅ Commit successful!"
echo ""
echo "📤 To push, run:"
echo "   git push origin $BRANCH_NAME"
echo ""
echo "🔗 Then create a PR on GitHub"

