#!/usr/bin/env python3
"""
Quick benchmark - skips Go bench, just HTTP tests
Won't hang on Windows
"""

import subprocess
import requests
import time
import os
import json
import threading
import sys

BINARY = "./sndv-kv.exe"
URL = "http://localhost:8080"

def start_server():
    print("Starting server...")
    proc = subprocess.Popen([BINARY, "-config", "config_fast.json"],
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    
    token = None
    for _ in range(50):
        line = proc.stdout.readline()
        if "ADMIN TOKEN:" in line:
            token = line.split("ADMIN TOKEN:")[1].strip()
            threading.Thread(target=lambda: [_ for _ in proc.stdout], daemon=True).start()
            time.sleep(0.5)
            break
        time.sleep(0.1)
    
    return proc, token

def bench(token, count, batch_size=1):
    headers = {"Authorization": token}
    success = 0
    start = time.time()
    
    if batch_size == 1:
        for i in range(count):
            r = requests.post(f"{URL}/put", 
                json={"key": f"k{i}", "value": "x"*64, "ttl": 0},
                headers=headers)
            if r.status_code == 201: success += 1
    else:
        batches = count // batch_size
        for i in range(batches):
            items = [{"key": f"b{i}_{j}", "value": "x"*64, "ttl": 0} 
                     for j in range(batch_size)]
            r = requests.post(f"{URL}/batch", json={"items": items}, headers=headers)
            if r.status_code == 201: success += batch_size
    
    return success / (time.time() - start)

def main():
    print("Building...")
    subprocess.run(["go", "build", "-o", BINARY, "cmd/server/main.go"],
        check=True, env={**os.environ, "CGO_ENABLED": "0"})
    
    proc, token = start_server()
    if not proc: return 1
    
    try:
        print("\nSingle ops (200)...")
        single = bench(token, 200, 1)
        print(f"  {single:,.0f} TPS")
        
        print("\nBatch ops (1000)...")
        batch = bench(token, 1000, 100)
        print(f"  {batch:,.0f} TPS")
        
        print(f"\n{'='*40}")
        print(f"Single: {single:>10,.0f} TPS")
        print(f"Batch:  {batch:>10,.0f} TPS")
        print(f"{'='*40}")
        
        with open("results.json", "w") as f:
            json.dump({"single": single, "batch": batch}, f)
        print("\nSaved: results.json")
        
    finally:
        proc.terminate()
        proc.wait()

if __name__ == "__main__":
    sys.exit(main())