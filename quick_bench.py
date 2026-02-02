#!/usr/bin/env python3
"""
SNDV-KV Performance Benchmark - WORKING VERSION
Parallel client with connection pooling and proper timeouts
"""

import requests
import time
import json
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
import statistics

# Global session with connection pooling
SESSION = None

def init_session():
    """Initialize session with connection pooling"""
    global SESSION
    SESSION = requests.Session()
    # Connection pool settings
    adapter = requests.adapters.HTTPAdapter(
        pool_connections=100,
        pool_maxsize=100,
        max_retries=0,
        pool_block=False
    )
    SESSION.mount('http://', adapter)
    SESSION.mount('https://', adapter)

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
            'min': min(sorted_lat),
            'max': max(sorted_lat),
            'avg': statistics.mean(sorted_lat),
            'p50': statistics.median(sorted_lat),
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
        for _ in range(5):
            try:
                resp = requests.get("http://localhost:8080/metrics", timeout=1)
                if resp.status_code == 200:
                    print("✅ Server started\n")
                    return proc
            except:
                time.sleep(0.5)
        
        print("❌ Server failed to start")
        proc.kill()
        return None
        
    except Exception as e:
        print(f"❌ Error starting server: {e}")
        return None

def single_put_request(i):
    """Execute a single PUT request"""
    start = time.perf_counter()
    try:
        resp = SESSION.post(
            "http://localhost:8080/put",
            json={"key": f"key{i}", "value": "x" * 64, "ttl": 0},
            timeout=2
        )
        latency_ms = (time.perf_counter() - start) * 1000
        return (resp.status_code == 201, latency_ms)
    except:
        latency_ms = (time.perf_counter() - start) * 1000
        return (False, latency_ms)

def batch_put_request(batch_id, batch_size):
    """Execute a batch PUT request"""
    items = [
        {"key": f"batch_{batch_id}_{i}", "value": "x" * 64, "ttl": 0}
        for i in range(batch_size)
    ]
    
    start = time.perf_counter()
    try:
        resp = SESSION.post(
            "http://localhost:8080/batch",
            json={"items": items},
            timeout=3
        )
        latency_ms = (time.perf_counter() - start) * 1000
        return (batch_size if resp.status_code == 201 else 0, latency_ms)
    except:
        latency_ms = (time.perf_counter() - start) * 1000
        return (0, latency_ms)

def benchmark_single_parallel(count=2000, workers=20):
    """Benchmark single PUT operations with parallel workers"""
    metrics = BenchmarkMetrics()
    
    print(f"Running Single PUT Benchmark ({count:,} items, {workers} workers)...")
    
    start_time = time.perf_counter()
    
    with ThreadPoolExecutor(max_workers=workers) as executor:
        futures = [executor.submit(single_put_request, i) for i in range(count)]
        
        completed = 0
        for future in as_completed(futures):
            success, latency = future.result()
            if success:
                metrics.add_success(latency)
            else:
                metrics.add_error()
            
            completed += 1
            if completed % 500 == 0:
                print(f"  Progress: {completed}/{count}")
    
    duration = time.perf_counter() - start_time
    stats = metrics.get_stats()
    tps = metrics.success / duration if duration > 0 else 0
    
    print(f"  → {tps:.0f} TPS")
    if stats:
        print(f"     Latency: min={stats['min']:.1f}ms, avg={stats['avg']:.1f}ms, p95={stats['p95']:.1f}ms, max={stats['max']:.1f}ms")
    if metrics.errors > 0:
        print(f"     Errors: {metrics.errors}")
    
    return tps

def benchmark_batch_parallel(total_items=20000, batch_size=100, workers=10):
    """Benchmark batch PUT operations with parallel workers"""
    num_batches = total_items // batch_size
    metrics = BenchmarkMetrics()
    
    print(f"\nRunning Batch PUT Benchmark ({total_items:,} items, batches of {batch_size}, {workers} workers)...")
    
    start_time = time.perf_counter()
    total_success = 0
    
    with ThreadPoolExecutor(max_workers=workers) as executor:
        futures = [
            executor.submit(batch_put_request, i, batch_size)
            for i in range(num_batches)
        ]
        
        completed = 0
        for future in as_completed(futures):
            success_count, latency = future.result()
            if success_count > 0:
                metrics.add_success(latency)
                total_success += success_count
            else:
                metrics.add_error()
            
            completed += 1
            if completed % 50 == 0:
                print(f"  Progress: {completed}/{num_batches} batches")
    
    duration = time.perf_counter() - start_time
    stats = metrics.get_stats()
    tps = total_success / duration if duration > 0 else 0
    
    print(f"  → {tps:.0f} TPS")
    if stats:
        print(f"     Batch latency: min={stats['min']:.1f}ms, avg={stats['avg']:.1f}ms, p95={stats['p95']:.1f}ms, max={stats['max']:.1f}ms")
    if metrics.errors > 0:
        print(f"     Failed batches: {metrics.errors}")
    
    return tps

def save_results(single_tps, batch_tps):
    """Save results to JSON"""
    results = {
        "single": single_tps,
        "batch": batch_tps
    }
    
    with open("results.json", "w") as f:
        json.dump(results, f, indent=2)
    
    print(f"\n✅ Saved results.json")

def main():
    print("="*70)
    print("SNDV-KV PERFORMANCE BENCHMARK")
    print("="*70)
    
    # Build server
    if not build_server():
        sys.exit(1)
    
    # Start server
    server_proc = start_server()
    if not server_proc:
        sys.exit(1)
    
    # Initialize session
    init_session()
    
    try:
        # Run benchmarks with reasonable defaults
        single_tps = benchmark_single_parallel(count=2000, workers=20)
        batch_tps = benchmark_batch_parallel(total_items=20000, batch_size=100, workers=10)
        
        # Summary
        print(f"\n{'='*70}")
        print(f"Single:        {single_tps:,.0f} TPS")
        print(f"Batch:      {batch_tps:,.0f} TPS")
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

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\n❌ Interrupted")
        sys.exit(1)
    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)