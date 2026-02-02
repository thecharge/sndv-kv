#!/usr/bin/env python3
"""
SNDV-KV Performance Benchmark
Parallel client with detailed metrics
"""

import requests
import time
import json
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
from collections import defaultdict
import statistics

class BenchmarkMetrics:
    def __init__(self):
        self.latencies = []
        self.errors = 0
        self.success = 0
        
    def add_success(self, latency_ms):
        self.success += 1
        self.latencies.append(latency_ms)
    
    def add_error(self):
        self.errors += 1
    
    def get_stats(self):
        if not self.latencies:
            return None
        
        sorted_lat = sorted(self.latencies)
        return {
            'count': len(sorted_lat),
            'min': min(sorted_lat),
            'max': max(sorted_lat),
            'avg': statistics.mean(sorted_lat),
            'median': statistics.median(sorted_lat),
            'p95': sorted_lat[int(len(sorted_lat) * 0.95)] if len(sorted_lat) > 20 else sorted_lat[-1],
            'p99': sorted_lat[int(len(sorted_lat) * 0.99)] if len(sorted_lat) > 100 else sorted_lat[-1],
        }

def build_server():
    """Build the server"""
    print("Building server...")
    build_cmd = ["go", "build", "-o", "sndv-kv.exe", "cmd/server/main.go"]
    
    if subprocess.run(build_cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode != 0:
        print("❌ Build failed")
        return False
    
    print("✅ Build successful")
    return True

def start_server():
    """Start the server process"""
    print("Starting server...")
    try:
        proc = subprocess.Popen(
            ["./sndv-kv.exe", "-config", "config_fast.json"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL
        )
        
        # Wait for server to be ready
        time.sleep(2)
        
        # Check if it's alive
        try:
            resp = requests.get("http://localhost:8080/metrics", timeout=1)
            if resp.status_code == 200:
                print("✅ Server started")
                return proc
        except:
            pass
        
        print("❌ Server failed to start")
        proc.kill()
        return None
        
    except Exception as e:
        print(f"❌ Error starting server: {e}")
        return None

def single_put_request(i, url):
    """Execute a single PUT request"""
    start = time.perf_counter()
    try:
        resp = requests.post(
            url,
            json={"key": f"key{i}", "value": "x" * 64, "ttl": 0},
            timeout=5
        )
        latency_ms = (time.perf_counter() - start) * 1000
        return (resp.status_code == 201, latency_ms)
    except Exception as e:
        latency_ms = (time.perf_counter() - start) * 1000
        return (False, latency_ms)

def batch_put_request(batch_id, batch_size, url):
    """Execute a batch PUT request"""
    items = [
        {"key": f"batch_{batch_id}_{i}", "value": "x" * 64, "ttl": 0}
        for i in range(batch_size)
    ]
    
    start = time.perf_counter()
    try:
        resp = requests.post(
            f"{url}/batch",
            json={"items": items},
            timeout=10
        )
        latency_ms = (time.perf_counter() - start) * 1000
        return (batch_size if resp.status_code == 201 else 0, latency_ms)
    except:
        latency_ms = (time.perf_counter() - start) * 1000
        return (0, latency_ms)

def benchmark_single_parallel(count=5000, workers=50):
    """Benchmark single PUT operations with parallel workers"""
    url = "http://localhost:8080/put"
    metrics = BenchmarkMetrics()
    
    print(f"\n{'='*70}")
    print(f"SINGLE PUT BENCHMARK (Parallel)")
    print(f"{'='*70}")
    print(f"  Operations: {count:,}")
    print(f"  Workers: {workers}")
    
    start_time = time.perf_counter()
    
    with ThreadPoolExecutor(max_workers=workers) as executor:
        futures = [executor.submit(single_put_request, i, url) for i in range(count)]
        
        for future in as_completed(futures):
            success, latency = future.result()
            if success:
                metrics.add_success(latency)
            else:
                metrics.add_error()
    
    duration = time.perf_counter() - start_time
    
    # Calculate stats
    stats = metrics.get_stats()
    tps = metrics.success / duration if duration > 0 else 0
    
    print(f"\n  Results:")
    print(f"    Success: {metrics.success:,}/{count:,}")
    print(f"    Errors: {metrics.errors:,}")
    print(f"    Duration: {duration:.2f}s")
    print(f"    Throughput: {tps:.0f} TPS")
    
    if stats:
        print(f"\n  Latency (ms):")
        print(f"    Min: {stats['min']:.2f}")
        print(f"    Avg: {stats['avg']:.2f}")
        print(f"    Median: {stats['median']:.2f}")
        print(f"    P95: {stats['p95']:.2f}")
        print(f"    P99: {stats['p99']:.2f}")
        print(f"    Max: {stats['max']:.2f}")
    
    return tps

def benchmark_batch_parallel(total_items=50000, batch_size=100, workers=20):
    """Benchmark batch PUT operations with parallel workers"""
    url = "http://localhost:8080"
    num_batches = total_items // batch_size
    metrics = BenchmarkMetrics()
    
    print(f"\n{'='*70}")
    print(f"BATCH PUT BENCHMARK (Parallel)")
    print(f"{'='*70}")
    print(f"  Total items: {total_items:,}")
    print(f"  Batch size: {batch_size}")
    print(f"  Batches: {num_batches}")
    print(f"  Workers: {workers}")
    
    start_time = time.perf_counter()
    total_success = 0
    
    with ThreadPoolExecutor(max_workers=workers) as executor:
        futures = [
            executor.submit(batch_put_request, i, batch_size, url)
            for i in range(num_batches)
        ]
        
        for future in as_completed(futures):
            success_count, latency = future.result()
            if success_count > 0:
                metrics.add_success(latency)
                total_success += success_count
            else:
                metrics.add_error()
    
    duration = time.perf_counter() - start_time
    
    # Calculate stats
    stats = metrics.get_stats()
    tps = total_success / duration if duration > 0 else 0
    
    print(f"\n  Results:")
    print(f"    Success: {total_success:,}/{total_items:,} items")
    print(f"    Failed batches: {metrics.errors:,}")
    print(f"    Duration: {duration:.2f}s")
    print(f"    Throughput: {tps:.0f} TPS")
    print(f"    Batches/sec: {metrics.success / duration:.0f}")
    
    if stats:
        print(f"\n  Batch Latency (ms):")
        print(f"    Min: {stats['min']:.2f}")
        print(f"    Avg: {stats['avg']:.2f}")
        print(f"    Median: {stats['median']:.2f}")
        print(f"    P95: {stats['p95']:.2f}")
        print(f"    P99: {stats['p99']:.2f}")
        print(f"    Max: {stats['max']:.2f}")
    
    return tps

def get_server_metrics():
    """Fetch server metrics"""
    try:
        resp = requests.get("http://localhost:8080/metrics", timeout=2)
        if resp.status_code == 200:
            return resp.json()
    except:
        pass
    return None

def print_server_metrics():
    """Print server metrics if available"""
    metrics = get_server_metrics()
    if metrics:
        print(f"\n{'='*70}")
        print("SERVER METRICS")
        print(f"{'='*70}")
        
        if 'write_ops' in metrics:
            print(f"  Total writes: {metrics['write_ops']:,}")
        if 'read_ops' in metrics:
            print(f"  Total reads: {metrics['read_ops']:,}")
        if 'memtable_size' in metrics:
            print(f"  MemTable size: {metrics['memtable_size']:,} bytes")
        if 'immutable_count' in metrics:
            print(f"  Immutable tables: {metrics['immutable_count']}")

def save_results(single_tps, batch_tps):
    """Save results to JSON"""
    results = {
        "single": single_tps,
        "batch": batch_tps,
        "timestamp": time.time()
    }
    
    with open("results.json", "w") as f:
        json.dump(results, f, indent=2)
    
    print(f"\n✅ Results saved to results.json")

def main():
    print("="*70)
    print("SNDV-KV PARALLEL PERFORMANCE BENCHMARK")
    print("="*70)
    
    # Build server
    if not build_server():
        sys.exit(1)
    
    # Start server
    server_proc = start_server()
    if not server_proc:
        sys.exit(1)
    
    try:
        # Run benchmarks
        single_tps = benchmark_single_parallel(count=5000, workers=50)
        batch_tps = benchmark_batch_parallel(total_items=50000, batch_size=100, workers=20)
        
        # Print server metrics
        print_server_metrics()
        
        # Summary
        print(f"\n{'='*70}")
        print("SUMMARY")
        print(f"{'='*70}")
        print(f"  Single PUT: {single_tps:,.0f} TPS")
        print(f"  Batch PUT:  {batch_tps:,.0f} TPS")
        print(f"{'='*70}")
        
        # Save results
        save_results(single_tps, batch_tps)
        
    finally:
        # Stop server
        print("\nStopping server...")
        server_proc.terminate()
        try:
            server_proc.wait(timeout=5)
        except:
            server_proc.kill()
        print("✅ Server stopped")

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\n❌ Interrupted by user")
        sys.exit(1)
    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)