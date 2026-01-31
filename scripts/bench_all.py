#!/usr/bin/env python3
import json
import subprocess
import time
import platform
from datetime import datetime
from pathlib import Path

def run_command(cmd):
    """Run command and return output"""
    result = subprocess.run(cmd, shell=True, capture_output=True, text=True)
    return result.stdout

def get_binary_name():
    """Get correct binary name for platform"""
    if platform.system() == "Windows":
        return "sndv-kv-bench.exe"
    return "sndv-kv-bench"

def main():
    version = run_command("git describe --tags --always --dirty").strip()
    timestamp = datetime.now()
    
    print("=" * 60)
    print(f"SNDV-KV Benchmark Suite - {version}")
    print(f"Platform: {platform.system()} {platform.machine()}")
    print("=" * 60)
    
    results = {
        "version": version,
        "timestamp": timestamp.isoformat(),
        "platform": platform.system(),
        "arch": platform.machine(),
        "metrics": {}
    }
    
    # 1. Build benchmark
    print("\n🔨 Build Metrics...")
    build_start = time.time()
    
    binary_name = get_binary_name()
    build_cmd = f"go build -ldflags=\"-s -w\" -o {binary_name} cmd/server/main.go"
    
    build_result = subprocess.run(build_cmd, shell=True, capture_output=True, text=True)
    if build_result.returncode != 0:
        print(f"❌ Build failed: {build_result.stderr}")
        return 1
    
    build_time = time.time() - build_start
    
    binary_path = Path(binary_name)
    if not binary_path.exists():
        print(f"❌ Binary not found: {binary_name}")
        return 1
    
    binary_size = binary_path.stat().st_size
    
    results["metrics"]["build_time_sec"] = round(build_time, 2)
    results["metrics"]["binary_size_mb"] = round(binary_size / 1024 / 1024, 2)
    
    print(f"   Build time: {build_time:.2f}s")
    print(f"   Binary size: {binary_size / 1024 / 1024:.2f} MB")
    
    # 2. Go benchmarks (disable CGO to avoid gcc requirement)
    print("\n📊 Go Benchmarks...")
    
    # Set CGO_ENABLED=0 for cross-platform compatibility
    env = {"CGO_ENABLED": "0"}
    bench_cmd = "go test -bench=BenchmarkEngineWriteParallel -benchmem -benchtime=5s ./internal/"
    
    bench_result = subprocess.run(
        bench_cmd, 
        shell=True, 
        capture_output=True, 
        text=True,
        env={**subprocess.os.environ, **env}
    )
    bench_output = bench_result.stdout
    
    # Parse benchmark output
    for line in bench_output.split('\n'):
        if 'BenchmarkEngineWriteParallel' in line and 'ns/op' in line:
            parts = line.split()
            try:
                # Find ns/op value
                for i, part in enumerate(parts):
                    if 'ns/op' in part:
                        ns_per_op = float(parts[i-1])
                        ops_per_sec = int(1_000_000_000 / ns_per_op)
                        
                        results["metrics"]["write_ops_per_sec"] = ops_per_sec
                        results["metrics"]["write_ns_per_op"] = ns_per_op
                        
                        print(f"   Ops/sec: {ops_per_sec:,}")
                        print(f"   Latency: {ns_per_op / 1000:.2f} μs")
                        break
                
                # Find B/op value
                for i, part in enumerate(parts):
                    if 'B/op' in part:
                        bytes_per_op = int(parts[i-1])
                        results["metrics"]["bytes_per_op"] = bytes_per_op
                        break
                
                # Find allocs/op value
                for i, part in enumerate(parts):
                    if 'allocs/op' in part:
                        allocs_per_op = int(parts[i-1])
                        results["metrics"]["allocs_per_op"] = allocs_per_op
                        print(f"   Allocs: {allocs_per_op}")
                        break
                        
            except (ValueError, IndexError) as e:
                print(f"   ⚠️  Could not parse benchmark line: {line}")
    
    # 3. Test coverage (also disable CGO)
    print("\n🧪 Test Coverage...")
    
    coverage_cmd = "go test ./... -coverprofile=coverage.out -covermode=atomic"
    coverage_result = subprocess.run(
        coverage_cmd,
        shell=True,
        capture_output=True,
        text=True,
        env={**subprocess.os.environ, **env}
    )
    
    if coverage_result.returncode == 0:
        coverage_output = run_command("go tool cover -func=coverage.out")
        
        # Parse coverage
        for line in coverage_output.split('\n'):
            if 'total:' in line or 'total' in line.lower():
                parts = line.split()
                if len(parts) >= 2:
                    coverage = parts[-1]
                    results["metrics"]["test_coverage"] = coverage
                    print(f"   Coverage: {coverage}")
                    break
    else:
        print(f"   ⚠️  Tests failed or incomplete")
        results["metrics"]["test_coverage"] = "N/A"
    
    # 4. Save results
    report_dir = Path("benchmark_reports")
    report_dir.mkdir(exist_ok=True)
    
    filename = report_dir / f"{version}_{timestamp.strftime('%Y%m%d_%H%M%S')}.json"
    with open(filename, 'w') as f:
        json.dump(results, f, indent=2)
    
    print(f"\n📁 Results saved: {filename}")
    print("=" * 60)
    
    # Cleanup
    if binary_path.exists():
        binary_path.unlink()
    
    return 0

if __name__ == "__main__":
    import sys
    sys.exit(main())