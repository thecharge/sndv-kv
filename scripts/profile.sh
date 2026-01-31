#!/bin/bash
set -e

REPORT_DIR="./benchmark_reports/profile_$(date +%Y%m%d_%H%M%S)"
mkdir -p "${REPORT_DIR}"

# CPU Profile
echo "🔥 CPU Profiling..."
go test -bench=BenchmarkEngineWriteParallel \
    -cpuprofile="${REPORT_DIR}/cpu.prof" \
    -memprofile="${REPORT_DIR}/mem.prof" \
    -benchtime=10s \
    ./internal/

# Generate reports
go tool pprof -text -nodecount=20 "${REPORT_DIR}/cpu.prof" > "${REPORT_DIR}/cpu_top20.txt"
go tool pprof -text -nodecount=20 -alloc_space "${REPORT_DIR}/mem.prof" > "${REPORT_DIR}/mem_top20.txt"

echo ""
echo "=== Top CPU Consumers ==="
head -n 15 "${REPORT_DIR}/cpu_top20.txt"

echo ""
echo "=== Top Memory Allocators ==="
head -n 15 "${REPORT_DIR}/mem_top20.txt"

echo ""
echo "📁 Full profiles: ${REPORT_DIR}"