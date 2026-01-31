#!/usr/bin/env python3
import json
from pathlib import Path
from typing import List

def load_all_results():
    """Load all benchmark results"""
    results = []
    for file in sorted(Path("benchmark_reports").glob("*.json")):
        try:
            with open(file) as f:
                data = json.load(f)
                results.append(data)
        except:
            pass
    
    results.sort(key=lambda x: x.get('timestamp', ''))
    return results

def print_comparison_table(results: List[dict]):
    """Print comparison table"""
    if len(results) < 2:
        print("Need at least 2 benchmark results")
        return
    
    print("=" * 100)
    print("SNDV-KV Version Comparison (Last 5 Versions)")
    print("=" * 100)
    
    # Take last 5
    recent = results[-5:]
    
    # Header
    versions = [r['version'][:15] for r in recent]
    print(f"{'Metric':<30}", end="")
    for v in versions:
        print(f"{v:>18}", end="")
    print()
    print("-" * 100)
    
    # Metrics
    def print_metric(name, key, formatter=lambda x: f"{x:,.0f}"):
        print(f"{name:<30}", end="")
        prev = None
        for r in recent:
            val = r.get('metrics', {}).get(key, 0)
            val_str = formatter(val)
            
            # Change indicator
            if prev is not None and isinstance(val, (int, float)):
                if val > prev * 1.05:  # 5% improvement
                    val_str += " ⬆️"
                elif val < prev * 0.95:  # 5% regression
                    val_str += " ⬇️"
            
            print(f"{val_str:>18}", end="")
            prev = val
        print()
    
    # Build metrics
    print_metric("Build Time (s)", "build_time_sec", lambda x: f"{x:.2f}")
    print_metric("Binary Size (MB)", "binary_size_mb", lambda x: f"{x:.2f}")
    print()
    
    # Performance metrics
    print_metric("Write Ops/sec", "write_ops_per_sec")
    print_metric("Latency (μs)", "write_ns_per_op", lambda x: f"{x/1000:.2f}")
    print_metric("Allocs/op", "allocs_per_op")
    print()
    
    # Test metrics
    print_metric("Coverage", "test_coverage", lambda x: f"{x}")
    
    print("=" * 100)
    
    # Progress chart
    print("\nPerformance Progress (Write Ops/sec)")
    print("-" * 100)
    
    max_ops = max(r.get('metrics', {}).get('write_ops_per_sec', 0) for r in recent)
    for r in recent:
        version = r['version'][:15]
        ops = r.get('metrics', {}).get('write_ops_per_sec', 0)
        bar_len = int((ops / max_ops) * 50) if max_ops > 0 else 0
        bar = "█" * bar_len
        print(f"{version:<15} {ops:>10,} ops/sec {bar}")
    
    print("=" * 100)
    
    # Regression warnings
    if len(recent) >= 2:
        current = recent[-1]
        previous = recent[-2]
        
        curr_ops = current.get('metrics', {}).get('write_ops_per_sec', 0)
        prev_ops = previous.get('metrics', {}).get('write_ops_per_sec', 0)
        
        if prev_ops > 0:
            change = ((curr_ops - prev_ops) / prev_ops) * 100
            
            print("\nRegression Check:")
            if change < -10:
                print(f"⚠️  REGRESSION: Performance dropped {abs(change):.1f}%")
            elif change > 10:
                print(f"✅ IMPROVEMENT: Performance increased {change:.1f}%")
            else:
                print(f"📊 STABLE: Performance within ±10% ({change:+.1f}%)")

def main():
    results = load_all_results()
    
    if not results:
        print("No benchmark results found.")
        print("Run: python3 scripts/bench_all.py")
        return
    
    print_comparison_table(results)

if __name__ == "__main__":
    main()