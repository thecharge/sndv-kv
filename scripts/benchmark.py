#!/usr/bin/env python3
"""
SNDV-KV: Quick Benchmark Suite
Fast, isolated, beautiful results.

Usage: python benchmark.py
"""

import subprocess
import requests
import time
import os
import json
import threading
import sys
import shutil
import statistics
from datetime import datetime
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

# ============================================================================
# CONFIGURATION
# ============================================================================

IS_WINDOWS = os.name == 'nt'
BINARY = "./sndv-kv.exe" if IS_WINDOWS else "./sndv-kv"
BENCHMARK_DIR = "./benchmark_run"
DATA_DIR = f"{BENCHMARK_DIR}/data"
LOG_DIR = f"{BENCHMARK_DIR}/logs"
SERVER_URL = "http://localhost:8080"

# Test parameters - keep small for speed
WARMUP_OPS = 50
SINGLE_OPS = 500
BATCH_OPS = 2000
BATCH_SIZE = 100
CONCURRENT_CLIENTS = 5

# ============================================================================
# COLORS
# ============================================================================

class C:
    H = '\033[95m'
    B = '\033[94m'
    C = '\033[96m'
    G = '\033[92m'
    Y = '\033[93m'
    R = '\033[91m'
    E = '\033[0m'
    BOLD = '\033[1m'

def header(text):
    print(f"\n{C.BOLD}{C.C}{'='*60}{C.E}")
    print(f"{C.BOLD}{C.C}{text.center(60)}{C.E}")
    print(f"{C.BOLD}{C.C}{'='*60}{C.E}\n")

def step(icon, text):
    print(f"{C.B}{icon}{C.E} {text}")

def ok(text):
    print(f"{C.G}✅ {text}{C.E}")

def fail(text):
    print(f"{C.R}❌ {text}{C.E}")

def metric(label, value, unit=""):
    print(f"   {label:<20} {C.G}{value:>12,}{unit}{C.E}")

# ============================================================================
# ENVIRONMENT
# ============================================================================

def create_env():
    """Create isolated environment"""
    step("🔧", "Creating isolated environment...")
    
    if os.path.exists(BENCHMARK_DIR):
        shutil.rmtree(BENCHMARK_DIR)
    
    os.makedirs(DATA_DIR, exist_ok=True)
    os.makedirs(LOG_DIR, exist_ok=True)
    
    ok("Environment ready")

def cleanup():
    """Cleanup but keep results"""
    step("🧹", "Cleaning up...")
    
    for item in [DATA_DIR, LOG_DIR]:
        if os.path.exists(item):
            try:
                shutil.rmtree(item)
            except:
                pass

def save_results(results):
    """Save results to JSON"""
    results_file = f"{BENCHMARK_DIR}/results.json"
    with open(results_file, 'w') as f:
        json.dump(results, f, indent=2)
    
    ok(f"Results: {results_file}")

def save_summary(results):
    """Save markdown summary"""
    summary = f"""# SNDV-KV Benchmark Results

**Date:** {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}  
**Version:** {results.get('version', 'unknown')}

## Performance

### Direct Engine (Go Benchmark)
- **Throughput:** {results.get('go_bench', {}).get('ops_per_sec', 0):,} ops/sec
- **Latency:** {results.get('go_bench', {}).get('ns_per_op', 0):.2f} ns/op

### HTTP API - Safe Mode (Durability)
- **Single:** {results.get('safe', {}).get('single', {}).get('tps', 0):,.0f} TPS
- **Batch:** {results.get('safe', {}).get('batch', {}).get('tps', 0):,.0f} TPS
- **p95 Latency:** {results.get('safe', {}).get('single', {}).get('p95_ms', 0):.2f} ms

### HTTP API - Fast Mode (In-Memory)
- **Single:** {results.get('fast', {}).get('single', {}).get('tps', 0):,.0f} TPS
- **Batch:** {results.get('fast', {}).get('batch', {}).get('tps', 0):,.0f} TPS

### Concurrent Load
- **Clients:** {CONCURRENT_CLIENTS}
- **Throughput:** {results.get('concurrent', {}).get('tps', 0):,.0f} TPS

## Build
- **Time:** {results.get('build', {}).get('time_sec', 0):.2f} seconds
- **Size:** {results.get('build', {}).get('size_mb', 0):.2f} MB
"""
    
    summary_file = f"{BENCHMARK_DIR}/SUMMARY.md"
    with open(summary_file, 'w') as f:
        f.write(summary)
    
    ok(f"Summary: {summary_file}")

# ============================================================================
# BUILD
# ============================================================================

def build():
    """Build server"""
    step("🔨", "Building...")
    
    start = time.time()
    
    try:
        subprocess.run(
            ["go", "build", "-ldflags=-s -w", "-o", BINARY, "cmd/server/main.go"],
            check=True,
            capture_output=True,
            env={**os.environ, "CGO_ENABLED": "0"}
        )
        
        build_time = time.time() - start
        size = os.path.getsize(BINARY) / 1024 / 1024
        
        ok(f"Built ({build_time:.2f}s, {size:.2f} MB)")
        
        return {'time_sec': build_time, 'size_mb': size}
        
    except subprocess.CalledProcessError as e:
        fail(f"Build failed: {e}")
        sys.exit(1)

# ============================================================================
# SERVER
# ============================================================================

class Server:
    def __init__(self, config_path):
        self.config_path = config_path
        self.process = None
        self.token = None
    
    def start(self):
        """Start server"""
        step("🚀", f"Starting server...")
        
        self.process = subprocess.Popen(
            [BINARY, "-config", self.config_path],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True
        )
        
        # Capture token with timeout
        start = time.time()
        while time.time() - start < 5:
            line = self.process.stdout.readline()
            if not line:
                break
            
            if "ADMIN TOKEN:" in line:
                self.token = line.split("ADMIN TOKEN:")[1].strip()
                
                # Drain stdout
                def drain():
                    for _ in self.process.stdout:
                        pass
                threading.Thread(target=drain, daemon=True).start()
                
                # Wait for ready
                time.sleep(0.5)
                ok(f"Server ready")
                return True
        
        fail("Token timeout")
        self.stop()
        return False
    
    def stop(self):
        """Stop server"""
        if self.process:
            self.process.terminate()
            try:
                self.process.wait(timeout=2)
            except:
                self.process.kill()

def create_config(filename, mode):
    """Create config file"""
    config = {
        "data_dir": DATA_DIR,
        "wal_path": f"{DATA_DIR}/wal.log",
        "log_dir": LOG_DIR,
        "port": 8080,
        "max_memtable_size": 64 * 1024 * 1024,
        "l0_compaction_trigger": 4,
        "auth_secret": "BENCHMARK",
        "durability": (mode == "SAFE"),
        "key_cache_size": 40000,
        "log_level": "ERROR"
    }
    
    path = f"{BENCHMARK_DIR}/{filename}"
    with open(path, 'w') as f:
        json.dump(config, f, indent=2)
    
    return path

# ============================================================================
# BENCHMARKS
# ============================================================================

def go_benchmark():
    """Run Go benchmark - with timeout"""
    step("📊", "Go benchmark (10s)...")
    
    try:
        # Short benchmark to avoid hanging
        result = subprocess.run(
            ["go", "test", "-bench=BenchmarkEngineWriteParallel", 
             "-benchtime=3s", "-timeout=30s", "./internal/"],
            capture_output=True,
            text=True,
            timeout=30,  # Hard timeout
            env={**os.environ, "CGO_ENABLED": "0"}
        )
        
        # Parse output
        for line in result.stdout.split('\n'):
            if 'BenchmarkEngineWriteParallel' in line and 'ns/op' in line:
                parts = line.split()
                
                ns_per_op = None
                for i, part in enumerate(parts):
                    if 'ns/op' in part and i > 0:
                        try:
                            ns_per_op = float(parts[i-1])
                            break
                        except:
                            pass
                
                if ns_per_op:
                    ops_per_sec = int(1_000_000_000 / ns_per_op)
                    metric("Ops/sec", ops_per_sec, "")
                    metric("Latency", ns_per_op, " ns")
                    
                    return {
                        'ops_per_sec': ops_per_sec,
                        'ns_per_op': ns_per_op
                    }
        
        # If we get here, parsing failed but benchmark ran
        print(f"   {C.Y}⚠️  Could not parse output{C.E}")
        return {}
        
    except subprocess.TimeoutExpired:
        print(f"   {C.Y}⚠️  Timeout - skipping{C.E}")
        return {}
    except Exception as e:
        print(f"   {C.Y}⚠️  Failed: {e}{C.E}")
        return {}

def warmup(token):
    """Warmup"""
    step("🔥", f"Warmup ({WARMUP_OPS} ops)...")
    
    headers = {"Authorization": token}
    for i in range(WARMUP_OPS):
        requests.post(f"{SERVER_URL}/put", 
            json={"key": f"w{i}", "value": "x", "ttl": 0},
            headers=headers)
    
    ok("Warmed up")

def single_ops(token, count):
    """Benchmark single operations"""
    step("📝", f"Single ops ({count})...")
    
    headers = {"Authorization": token}
    latencies = []
    success = 0
    
    start = time.time()
    
    for i in range(count):
        t0 = time.perf_counter()
        resp = requests.post(f"{SERVER_URL}/put",
            json={"key": f"s{i}", "value": "x"*64, "ttl": 0},
            headers=headers)
        lat = (time.perf_counter() - t0) * 1000
        
        latencies.append(lat)
        if resp.status_code == 201:
            success += 1
    
    dur = time.time() - start
    tps = success / dur
    
    latencies.sort()
    p50 = latencies[len(latencies)//2]
    p95 = latencies[int(len(latencies)*0.95)]
    p99 = latencies[int(len(latencies)*0.99)]
    
    metric("TPS", tps, "")
    metric("p50", p50, " ms")
    metric("p95", p95, " ms")
    
    return {
        'tps': tps,
        'p50_ms': p50,
        'p95_ms': p95,
        'p99_ms': p99,
        'success_rate': success/count
    }

def batch_ops(token, total, batch_size):
    """Benchmark batch operations"""
    step("📦", f"Batch ops ({total} items)...")
    
    headers = {"Authorization": token}
    success = 0
    batches = total // batch_size
    
    start = time.time()
    
    for i in range(batches):
        items = [{"key": f"b{i}_{j}", "value": "x"*64, "ttl": 0} 
                 for j in range(batch_size)]
        
        resp = requests.post(f"{SERVER_URL}/batch",
            json={"items": items},
            headers=headers)
        
        if resp.status_code == 201:
            success += batch_size
    
    dur = time.time() - start
    tps = success / dur
    
    metric("TPS", tps, "")
    
    return {
        'tps': tps,
        'success_rate': success/total
    }

def concurrent_load(token, clients, ops_per_client):
    """Concurrent load test"""
    step("🔀", f"Concurrent ({clients} clients)...")
    
    headers = {"Authorization": token}
    
    def worker(cid):
        success = 0
        for i in range(ops_per_client):
            resp = requests.post(f"{SERVER_URL}/put",
                json={"key": f"c{cid}_{i}", "value": "x"*64, "ttl": 0},
                headers=headers)
            if resp.status_code == 201:
                success += 1
        return success
    
    start = time.time()
    
    with ThreadPoolExecutor(max_workers=clients) as ex:
        futures = [ex.submit(worker, i) for i in range(clients)]
        results = [f.result() for f in as_completed(futures)]
    
    dur = time.time() - start
    total_success = sum(results)
    total_ops = clients * ops_per_client
    tps = total_success / dur
    
    metric("TPS", tps, "")
    metric("Success", (total_success/total_ops)*100, "%")
    
    return {
        'clients': clients,
        'total_ops': total_ops,
        'duration_sec': dur,
        'tps': tps,
        'success_rate': total_success/total_ops
    }

# ============================================================================
# MAIN
# ============================================================================

def main():
    header("SNDV-KV Quick Benchmark")
    
    # Version
    try:
        version = subprocess.check_output(
            ["git", "describe", "--tags", "--always", "--dirty"],
            text=True
        ).strip()
    except:
        version = "unknown"
    
    print(f"Version: {version}")
    print(f"Platform: {sys.platform}\n")
    
    results = {
        'version': version,
        'platform': sys.platform,
        'timestamp': datetime.now().isoformat()
    }
    
    # Setup
    header("Setup")
    create_env()
    results['build'] = build()
    
    # Go benchmark
    header("Go Benchmark")
    results['go_bench'] = go_benchmark()
    
    # HTTP - Safe
    header("HTTP - Safe Mode")
    config_safe = create_config("config_safe.json", "SAFE")
    server_safe = Server(config_safe)
    
    if server_safe.start():
        try:
            warmup(server_safe.token)
            results['safe'] = {
                'single': single_ops(server_safe.token, SINGLE_OPS),
                'batch': batch_ops(server_safe.token, BATCH_OPS, BATCH_SIZE)
            }
        finally:
            server_safe.stop()
            time.sleep(0.5)
    
    # HTTP - Fast
    header("HTTP - Fast Mode")
    
    # Clean between modes
    if os.path.exists(DATA_DIR):
        shutil.rmtree(DATA_DIR)
    os.makedirs(DATA_DIR)
    
    config_fast = create_config("config_fast.json", "FAST")
    server_fast = Server(config_fast)
    
    if server_fast.start():
        try:
            warmup(server_fast.token)
            results['fast'] = {
                'single': single_ops(server_fast.token, SINGLE_OPS),
                'batch': batch_ops(server_fast.token, BATCH_OPS, BATCH_SIZE)
            }
            results['concurrent'] = concurrent_load(
                server_fast.token, 
                CONCURRENT_CLIENTS,
                SINGLE_OPS // CONCURRENT_CLIENTS
            )
        finally:
            server_fast.stop()
    
    # Save
    header("Results")
    save_results(results)
    save_summary(results)
    
    # Summary
    header("Summary")
    print(f"\n{C.BOLD}Performance:{C.E}\n")
    print(f"  Direct:          {results.get('go_bench', {}).get('ops_per_sec', 0):>10,} ops/sec")
    print(f"  HTTP Safe:       {results.get('safe', {}).get('batch', {}).get('tps', 0):>10,.0f} TPS")
    print(f"  HTTP Fast:       {results.get('fast', {}).get('batch', {}).get('tps', 0):>10,.0f} TPS")
    print(f"  Concurrent:      {results.get('concurrent', {}).get('tps', 0):>10,.0f} TPS")
    
    print(f"\n{C.BOLD}Files:{C.E}\n")
    print(f"  {BENCHMARK_DIR}/results.json")
    print(f"  {BENCHMARK_DIR}/SUMMARY.md")
    
    # Cleanup
    cleanup()
    
    print(f"\n{C.G}✅ Done! Check {BENCHMARK_DIR}/ for results.{C.E}\n")

if __name__ == "__main__":
    main()