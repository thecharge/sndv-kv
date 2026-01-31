#!/usr/bin/env python3
"""
Human-perceptible latency benchmarks.
Answer: "How does this FEEL?"
"""

import time
import requests
import statistics
from typing import Callable, List

def measure_latency(func: Callable, iterations=100) -> dict:
    """Measure latency distribution"""
    latencies = []
    
    for _ in range(iterations):
        start = time.perf_counter()
        func()
        latency_ms = (time.perf_counter() - start) * 1000
        latencies.append(latency_ms)
    
    latencies.sort()
    
    return {
        'min': latencies[0],
        'p50': latencies[len(latencies) // 2],
        'p95': latencies[int(len(latencies) * 0.95)],
        'p99': latencies[int(len(latencies) * 0.99)],
        'max': latencies[-1],
        'avg': statistics.mean(latencies),
    }

def interpret_latency(operation: str, p95_ms: float):
    """Human interpretation"""
    print(f"\n   👤 How {operation} FEELS:")
    
    if p95_ms < 100:
        print(f"   ✅ INSTANT - Users won't notice")
        print(f"      Feels immediate and responsive")
    elif p95_ms < 300:
        print(f"   ⚡ FAST - Feels snappy")
        print(f"      Quick and pleasant to use")
    elif p95_ms < 1000:
        print(f"   🙂 OK - Noticeable but acceptable")
        print(f"      Slight lag, still usable")
    else:
        print(f"   😐 SLOW - Users will get impatient")
        print(f"      Feels sluggish")

def main():
    url = "http://localhost:8080"
    token = input("Auth token: ").strip()
    
    print("=" * 60)
    print("Human-Perceptible Latency Test")
    print("=" * 60)
    
    # Single write
    print("\n📝 Single Write (like clicking 'Save')")
    stats = measure_latency(
        lambda: requests.post(f"{url}/put", 
            json={"key": "test", "value": "val"},
            headers={"Authorization": token}),
        iterations=100
    )
    
    print(f"   p50: {stats['p50']:.1f} ms")
    print(f"   p95: {stats['p95']:.1f} ms")
    print(f"   p99: {stats['p99']:.1f} ms")
    interpret_latency("saving", stats['p95'])
    
    # Single read
    print("\n📖 Single Read (like loading a page)")
    stats = measure_latency(
        lambda: requests.get(f"{url}/get?key=test",
            headers={"Authorization": token}),
        iterations=100
    )
    
    print(f"   p50: {stats['p50']:.1f} ms")
    print(f"   p95: {stats['p95']:.1f} ms")
    print(f"   p99: {stats['p99']:.1f} ms")
    interpret_latency("loading", stats['p95'])
    
    print("\n" + "=" * 60)

if __name__ == "__main__":
    main()