import requests
import subprocess
import time
import sys
import os
import threading
import json

# --- CONFIGURATION ---
BINARY = "sndv-kv.exe" if os.name == 'nt' else "./sndv-kv"
PORT = 8080
CONFIG_FILE = "config_ref.json"

def print_step(msg): print(f"\n🔹 {msg}")
def print_success(msg): print(f"   ✅ {msg}")
def print_error(msg): print(f"   ❌ {msg}"); sys.exit(1)

# 1. Helper to capture the Auto-Generated Token
def start_server_and_get_token():
    print_step("Starting Server...")
    
    # Create a simple config on the fly
    config = {
        "port": PORT,
        "durability": True, # Safe mode by default
        "data_dir": "./data_example",
        "wal_path": "./data_example/wal.log",
        "auth_token": "", # Empty = Auto-generate
        "max_memtable_size": 10 * 1024 * 1024
    }
    with open(CONFIG_FILE, "w") as f:
        json.dump(config, f)

    # Start process
    proc = subprocess.Popen(
        [BINARY, "-config", CONFIG_FILE],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1
    )

    # Scrape stdout for token
    token = None
    start_time = time.time()
    
    while True:
        line = proc.stdout.readline()
        if not line: break
        if "ADMIN TOKEN:" in line:
            token = line.split("ADMIN TOKEN:")[1].strip()
            break
        if time.time() - start_time > 5:
            print_error("Server timed out waiting for token.")
            
    if not token:
        print_error("Could not scrape token.")
        
    print_success(f"Server started. Token acquired.")
    return proc, token

# --- MAIN CLIENT LOGIC ---
def main():
    # 1. Setup
    server_proc, token = start_server_and_get_token()
    base_url = f"http://localhost:{PORT}"
    headers = {"Authorization": token, "Content-Type": "application/json"}
    
    try:
        # 2. CREATE (Put)
        print_step("1. Writing Data (PUT)")
        payload = {"key": "user:100", "value": "John Doe", "ttl": 3600}
        resp = requests.post(f"{base_url}/put", json=payload, headers=headers)
        
        if resp.status_code == 201:
            print_success(f"Wrote key '{payload['key']}'")
        else:
            print_error(f"Put failed: {resp.text}")

        # 3. READ (Get)
        print_step("2. Reading Data (GET)")
        resp = requests.get(f"{base_url}/get?key=user:100", headers=headers)
        
        if resp.status_code == 200:
            data = resp.json()
            if data['val'] == "John Doe":
                print_success(f"Read back correct value: '{data['val']}'")
            else:
                print_error(f"Value mismatch! Got {data['val']}")
        else:
            print_error(f"Get failed: {resp.status_code}")

        # 4. UPDATE (Put again)
        print_step("3. Updating Data")
        payload["value"] = "Jane Doe"
        requests.post(f"{base_url}/put", json=payload, headers=headers)
        
        resp = requests.get(f"{base_url}/get?key=user:100", headers=headers)
        if resp.json()['val'] == "Jane Doe":
             print_success("Value updated successfully")

        # 5. DELETE
        print_step("4. Deleting Data")
        requests.post(f"{base_url}/delete?key=user:100", headers=headers)
        
        resp = requests.get(f"{base_url}/get?key=user:100", headers=headers)
        if resp.status_code == 404:
            print_success("Key correctly returned 404 Not Found")
        else:
            print_error("Key still exists after delete!")

        # 6. METRICS
        print_step("5. Checking Metrics")
        resp = requests.get(f"{base_url}/metrics")
        print(json.dumps(resp.json(), indent=2))

    finally:
        print_step("Cleaning up...")
        server_proc.terminate()
        server_proc.wait()
        if os.path.exists(CONFIG_FILE): os.remove(CONFIG_FILE)

if __name__ == "__main__":
    main()