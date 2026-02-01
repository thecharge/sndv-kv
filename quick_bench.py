#!/usr/bin/env python3
"""
Fixed Benchmark Script
- Uses requests.Session() for Keep-Alive (Fixes 2s delay/slow logs)
- Drains stdout properly (Fixes hang)
- Saves results.json
"""

import subprocess
import requests
import time
import os
import json
import threading
import sys
import signal

BINARY = "./sndv-kv.exe" if os.name == 'nt' else "./sndv-kv"
URL = "http://localhost:8080"

def stream_discarder(pipe):
    """Continuously reads and discards output to prevent buffer blocking"""
    try:
        for _ in pipe:
            pass
    except (ValueError, OSError):
        pass

def start_server():
    print("Starting server...")
    # Ensure config exists
    if not os.path.exists("config_fast.json"):
        print("Error: config_fast.json missing. Run the previous setup or create it.")
        return None, None

    proc = subprocess.Popen(
        [BINARY, "-config", "config_fast.json"],
        stdout=subprocess.PIPE, 
        stderr=subprocess.STDOUT, 
        text=True,
        bufsize=1 # Line buffered
    )
    
    token = None
    # Read startup lines looking for token
    start_wait = time.time()
    while time.time() - start_wait < 5:
        line = proc.stdout.readline()
        if not line: break
        if "ADMIN TOKEN:" in line:
            token = line.split("ADMIN TOKEN:")[1].strip()
            # Start background drainer immediately after getting token
            threading.Thread(target=stream_discarder, args=(proc.stdout,), daemon=True).start()
            break
        time.sleep(0.01)
    
    if not token:
        print("Failed to capture token (Server might have failed to start)")
        proc.kill()
        return None, None
        
    return proc, token

def bench(token, count, batch_size=1):
    headers = {"Authorization": token, "Connection": "keep-alive"}
    session = requests.Session()
    
    # Pre-generate payloads to measure I/O only
    single_payload = {"key": "bench", "value": "x"*64, "ttl": 0}
    
    start = time.time()
    success = 0
    
    if batch_size == 1:
        url = f"{URL}/put"
        for i in range(count):
            # Mutate key slightly to avoid client-side caching effects if any (rare in requests)
            single_payload["key"] = f"k{i}"
            try:
                r = session.post(url, json=single_payload, headers=headers)
                if r.status_code == 201: success += 1
            except Exception as e:
                print(f"Req failed: {e}")
    else:
        url = f"{URL}/batch"
        batches = count // batch_size
        for i in range(batches):
            items = [{"key": f"b{i}_{j}", "value": "x"*64, "ttl": 0} for j in range(batch_size)]
            try:
                r = session.post(url, json={"items": items}, headers=headers)
                if r.status_code == 201: success += batch_size
            except Exception as e:
                print(f"Batch failed: {e}")
    
    elapsed = time.time() - start
    return success / elapsed if elapsed > 0 else 0

def main():
    # Build first to ensure latest code
    print("Building...")
    build_cmd = ["go", "build", "-o", BINARY, "cmd/server/main.go"]
    if subprocess.run(build_cmd).returncode != 0:
        print("Build failed")
        return 1

    proc, token = start_server()
    if not proc: return 1
    
    results = {}
    
    try:
        # Warmup
        requests.get(f"{URL}/metrics", headers={"Authorization": token})
        
        print("\nRunning Single PUT Benchmark (1000 items)...")
        single_tps = bench(token, 1000, 1)
        print(f"-> {single_tps:,.0f} TPS")
        results["single"] = single_tps
        
        print("\nRunning Batch PUT Benchmark (50,000 items)...")
        batch_tps = bench(token, 50000, 100)
        print(f"-> {batch_tps:,.0f} TPS")
        results["batch"] = batch_tps
        
        print(f"\n{'='*30}")
        print(f"Single: {single_tps:>10,.0f} TPS")
        print(f"Batch:  {batch_tps:>10,.0f} TPS")
        print(f"{'='*30}")
        
        with open("results.json", "w") as f:
            json.dump(results, f, indent=4)
        print("\nSaved results.json")
        
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=2)
        except subprocess.TimeoutExpired:
            proc.kill()

if __name__ == "__main__":
    sys.exit(main())