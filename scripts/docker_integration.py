#!/usr/bin/env python3
"""
Integration test runner for Docker containers
Runs comprehensive tests and saves artifacts
"""

import requests
import time
import json
import os
import sys
from pathlib import Path

class IntegrationTest:
    def __init__(self, base_url, artifacts_dir):
        self.base_url = base_url
        self.artifacts_dir = Path(artifacts_dir)
        self.artifacts_dir.mkdir(parents=True, exist_ok=True)
        self.token = None
        self.results = {
            "test_case": os.getenv("TEST_CASE", "unknown"),
            "vcpu": os.getenv("VCPU", "unknown"),
            "memory": os.getenv("MEMORY", "unknown"),
            "timestamp": time.time(),
            "tests": {}
        }
    
    def wait_for_ready(self, timeout=30):
        """Wait for server"""
        start = time.time()
        while time.time() - start < timeout:
            try:
                resp = requests.get(f"{self.base_url}/health", timeout=1)
                if resp.status_code == 200:
                    return True
            except:
                pass
            time.sleep(0.5)
        return False
    
    def get_token_from_logs(self):
        """Read token from container logs"""
        # Read from mounted log file
        log_file = Path("/app/logs/server.log")
        if log_file.exists():
            content = log_file.read_text()
            for line in content.split('\n'):
                if "ADMIN TOKEN:" in line:
                    self.token = line.split("ADMIN TOKEN:")[1].strip()
                    return
        # Fallback for testing
        self.token = "test-token-placeholder"
    
    def test_happy_path(self):
        """Basic PUT -> GET test"""
        print("  [1/4] Happy path...")
        headers = {"Authorization": self.token}
        
        # PUT
        start = time.time()
        resp = requests.post(f"{self.base_url}/put",
            json={"key": "test", "value": "value", "ttl": 0},
            headers=headers)
        put_time = (time.time() - start) * 1000
        
        # GET
        start = time.time()
        resp = requests.get(f"{self.base_url}/get?key=test", headers=headers)
        get_time = (time.time() - start) * 1000
        
        passed = resp.status_code == 200 and resp.json().get("val") == "value"
        
        self.results["tests"]["happy_path"] = {
            "passed": passed,
            "put_ms": put_time,
            "get_ms": get_time
        }
        return passed
    
    def test_critical_path(self):
        """Batch operations with verification"""
        print("  [2/4] Critical path...")
        headers = {"Authorization": self.token}
        
        # Batch write
        items = [{"key": f"c{i}", "value": f"v{i}", "ttl": 0} for i in range(100)]
        start = time.time()
        resp = requests.post(f"{self.base_url}/batch", json={"items": items}, headers=headers)
        batch_time = (time.time() - start) * 1000
        
        # Verify
        verified = sum(1 for i in range(100) 
                      if requests.get(f"{self.base_url}/get?key=c{i}", headers=headers).status_code == 200)
        
        self.results["tests"]["critical_path"] = {
            "passed": verified == 100,
            "batch_ms": batch_time,
            "verified": verified
        }
        return verified == 100
    
    def benchmark_single(self, count=500):
        """Benchmark single ops"""
        print(f"  [3/4] Single benchmark ({count} ops)...")
        headers = {"Authorization": self.token}
        latencies = []
        success = 0
        
        start = time.time()
        for i in range(count):
            t0 = time.time()
            resp = requests.post(f"{self.base_url}/put",
                json={"key": f"s{i}", "value": "x"*64, "ttl": 0},
                headers=headers)
            latencies.append((time.time() - t0) * 1000)
            if resp.status_code == 201:
                success += 1
        
        duration = time.time() - start
        tps = success / duration
        latencies.sort()
        
        self.results["tests"]["benchmark_single"] = {
            "count": count,
            "success": success,
            "tps": tps,
            "p50_ms": latencies[len(latencies)//2],
            "p95_ms": latencies[int(len(latencies)*0.95)],
            "p99_ms": latencies[int(len(latencies)*0.99)]
        }
        return tps
    
    def benchmark_batch(self, total=2000, batch_size=100):
        """Benchmark batch ops"""
        print(f"  [4/4] Batch benchmark ({total} items)...")
        headers = {"Authorization": self.token}
        success = 0
        
        start = time.time()
        for i in range(total // batch_size):
            items = [{"key": f"b{i}_{j}", "value": "x"*64, "ttl": 0} for j in range(batch_size)]
            resp = requests.post(f"{self.base_url}/batch", json={"items": items}, headers=headers)
            if resp.status_code == 201:
                success += batch_size
        
        tps = success / (time.time() - start)
        
        self.results["tests"]["benchmark_batch"] = {
            "total": total,
            "success": success,
            "tps": tps
        }
        return tps
    
    def save_results(self):
        """Save all results"""
        # JSON
        with open(self.artifacts_dir / "integration_results.json", "w") as f:
            json.dump(self.results, f, indent=2)
        
        # Text summary
        with open(self.artifacts_dir / "SUMMARY.txt", "w") as f:
            f.write(f"Test Case: {self.results['test_case']}\n")
            f.write(f"Resources: {self.results['vcpu']} vCPU, {self.results['memory']}\n\n")
            for test_name, result in self.results["tests"].items():
                f.write(f"{test_name}:\n")
                for k, v in result.items():
                    f.write(f"  {k}: {v}\n")
                f.write("\n")
    
    def run_all(self):
        """Execute all tests"""
        print(f"\nRunning tests: {self.results['test_case']}")
        
        if not self.wait_for_ready():
            print("  ❌ Server timeout")
            self.results["error"] = "Server not ready"
            self.save_results()
            return False
        
        print("  ✅ Server ready")
        self.get_token_from_logs()
        
        # Run all tests
        try:
            self.test_happy_path()
            self.test_critical_path()
            self.benchmark_single(500)
            self.benchmark_batch(2000, 100)
            print("  ✅ All tests complete")
        except Exception as e:
            print(f"  ❌ Test failed: {e}")
            self.results["error"] = str(e)
            self.save_results()
            return False
        
        self.save_results()
        return True

if __name__ == "__main__":
    url = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8080"
    artifacts = sys.argv[2] if len(sys.argv) > 2 else "/artifacts"
    
    test = IntegrationTest(url, artifacts)
    sys.exit(0 if test.run_all() else 1)