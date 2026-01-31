#!/bin/bash
# Windows-compatible test coverage script

set -e

# Disable CGO (not needed for SNDV-KV)
export CGO_ENABLED=0

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "unknown")
TIMESTAMP=$(date +%Y%m%d_%H%M%S 2>/dev/null || echo $(date +%s))
REPORT_DIR="./benchmark_reports/${VERSION}_${TIMESTAMP}"

# Create report directory
mkdir -p "${REPORT_DIR}"

echo "=== SNDV-KV Test Suite ==="
echo "Version: ${VERSION}"
echo "CGO: Disabled (cross-platform)"
echo ""

# Run tests with coverage
echo "📊 Running tests with coverage..."
go test ./... -v \
    -coverprofile="${REPORT_DIR}/coverage.out" \
    -covermode=atomic \
    -timeout=5m \
    2>&1 | tee "${REPORT_DIR}/test_output.log"

# Check if tests passed
if [ ${PIPESTATUS[0]} -ne 0 ]; then
    echo "❌ Some tests failed"
else
    echo "✅ All tests passed"
fi

# Generate coverage HTML
if [ -f "${REPORT_DIR}/coverage.out" ]; then
    echo ""
    echo "📈 Generating coverage report..."
    go tool cover -html="${REPORT_DIR}/coverage.out" -o "${REPORT_DIR}/coverage.html"
    
    COVERAGE=$(go tool cover -func="${REPORT_DIR}/coverage.out" | grep total | awk '{print $3}')
    echo "Total Coverage: ${COVERAGE}"
    
    echo ""
    echo "✅ Tests complete: ${COVERAGE} coverage"
    echo "📁 HTML report: ${REPORT_DIR}/coverage.html"
else
    echo "❌ No coverage data generated"
fi