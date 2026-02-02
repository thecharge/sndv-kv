#!/usr/bin/env python3
"""
Profile the actual running server to find bottlenecks
"""

import subprocess
import time
import requests
import json
import sys

def start_server_with_profiling():
    """Start server with CPU profiling enabled"""
    print("Building server with profiling...")
    
    # Build
    result = subprocess.run(
        ["go", "build", "-o", "sndv-kv-profile.exe", "cmd/server/main.go"],
        capture_output=True,
        text=True
    )
    
    if result.returncode != 0:
        print(f"Build failed: {result.stderr}")
        return None
    
    print("Starting server with pprof...")
    
    # Start server (pprof should be enabled in config)
    proc = subprocess.Popen(
        ["./sndv-kv-profile.exe", "-config", "config_fast.json"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )
    
    # Wait for server to start
    time.sleep(3)
    
    # Check if it's alive
    try:
        resp = requests.get("http://localhost:8080/metrics", timeout=2)
        if resp.status_code == 200:
            print("✅ Server started")
            return proc
    except:
        pass
    
    print("❌ Server failed to start")
    proc.kill()
    return None

def run_load_test():
    """Generate load while profiling"""
    print("\n🔥 Running load test...")
    
    url = "http://localhost:8080"
    
    # Single operations
    print("Single operations (1000)...")
    start = time.time()
    success = 0
    
    for i in range(1000):
        try:
            resp = requests.post(
                f"{url}/put",
                json={"key": f"key{i}", "value": "x"*64, "ttl": 0},
                timeout=2
            )
            if resp.status_code == 201:
                success += 1
        except:
            pass
    
    duration = time.time() - start
    tps = success / duration
    print(f"  → {tps:.0f} TPS")
    
    # Batch operations
    print("Batch operations (10,000 items)...")
    start = time.time()
    success = 0
    
    for batch_num in range(100):
        items = [
            {"key": f"batch{batch_num}_{i}", "value": "x"*64, "ttl": 0}
            for i in range(100)
        ]
        try:
            resp = requests.post(
                f"{url}/batch",
                json={"items": items},
                timeout=5
            )
            if resp.status_code == 201:
                success += 100
        except:
            pass
    
    duration = time.time() - start
    tps = success / duration
    print(f"  → {tps:.0f} TPS")

def capture_profiles():
    """Capture CPU and memory profiles from pprof"""
    print("\n📊 Capturing profiles...")
    
    try:
        # CPU profile
        print("Capturing CPU profile (30 seconds)...")
        subprocess.run([
            "curl", "-o", "cpu.prof",
            "http://localhost:6060/debug/pprof/profile?seconds=30"
        ], timeout=35, capture_output=True)
        print("✅ CPU profile saved: cpu.prof")
        
        # Heap profile
        print("Capturing heap profile...")
        subprocess.run([
            "curl", "-o", "heap.prof",
            "http://localhost:6060/debug/pprof/heap"
        ], timeout=10, capture_output=True)
        print("✅ Heap profile saved: heap.prof")
        
        # Goroutine profile
        print("Capturing goroutine profile...")
        subprocess.run([
            "curl", "-o", "goroutine.prof",
            "http://localhost:6060/debug/pprof/goroutine"
        ], timeout=10, capture_output=True)
        print("✅ Goroutine profile saved: goroutine.prof")
        
        return True
        
    except Exception as e:
        print(f"❌ Profile capture failed: {e}")
        return False

def analyze_profiles():
    """Analyze profiles and show top functions"""
    print("\n🔍 Analyzing profiles...")
    
    print("\n" + "="*70)
    print("CPU PROFILE - Top 20 functions")
    print("="*70)
    
    result = subprocess.run([
        "go", "tool", "pprof", "-top", "-nodecount=20", "cpu.prof"
    ], capture_output=True, text=True)
    
    print(result.stdout)
    
    print("\n" + "="*70)
    print("MEMORY PROFILE - Top 20 allocations")
    print("="*70)
    
    result = subprocess.run([
        "go", "tool", "pprof", "-top", "-nodecount=20", "-alloc_space", "heap.prof"
    ], capture_output=True, text=True)
    
    print(result.stdout)

def main():
    # Start server
    proc = start_server_with_profiling()
    if not proc:
        sys.exit(1)
    
    try:
        # Run load test
        run_load_test()
        
        # Capture profiles during load
        if not capture_profiles():
            print("\n⚠️  Make sure pprof is enabled in config:")
            print('    "enable_pprof_profiling": true')
            sys.exit(1)
        
        # Analyze
        analyze_profiles()
        
        print("\n" + "="*70)
        print("DONE - Profile files saved:")
        print("  - cpu.prof (CPU profile)")
        print("  - heap.prof (Memory profile)")
        print("  - goroutine.prof (Goroutine profile)")
        print("\nTo view interactively:")
        print("  go tool pprof -http=:8081 cpu.prof")
        print("="*70)
        
    finally:
        # Stop server
        print("\nStopping server...")
        proc.terminate()
        proc.wait(timeout=5)

if __name__ == "__main__":
    main()