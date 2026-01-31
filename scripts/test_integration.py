import requests
import sys
import time

if len(sys.argv) < 2:
    print("Usage: python test.py <TOKEN>")
    sys.exit(1)

URL = "http://localhost:8080"
Headers = {"Authorization": sys.argv[1]}

def ok(msg): print(f"✅ {msg}")
def fail(msg): print(f"❌ {msg}"); sys.exit(1)

# 1. Put
r = requests.post(f"{URL}/put", json={"key":"A", "value":"1", "ttl": 5}, headers=Headers)
if r.status_code == 201: ok("Put A")
else: fail(f"Put failed: {r.text}")

# 2. Get
r = requests.get(f"{URL}/get?key=A", headers=Headers)
if r.json()['val'] == "1": ok("Get A")
else: fail("Get A mismatch")

# 3. Delete
requests.post(f"{URL}/delete?key=A", headers=Headers)
r = requests.get(f"{URL}/get?key=A", headers=Headers)
if r.status_code == 404: ok("Delete A (Tombstone)")
else: fail("Delete A failed")

# 4. TTL
requests.post(f"{URL}/put", json={"key":"B", "value":"2", "ttl": 2}, headers=Headers)
time.sleep(3)
r = requests.get(f"{URL}/get?key=B", headers=Headers)
if r.status_code == 404: ok("TTL Expired")
else: fail("TTL failed")

print("All System Tests Passed.")