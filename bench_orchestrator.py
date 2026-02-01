#!/usr/bin/env python3
"""
Master benchmark orchestrator
Runs all test cases with different resource configurations
"""

import subprocess
import time
import json
import shutil
from pathlib import Path
from datetime import datetime

class BenchOrchestrator:
    def __init__(self):
        self.run_id = datetime.now().strftime("%Y%m%d_%H%M%S")
        self.results_dir = Path(f"benchmark_results_{self.run_id}")
        self.configs_dir = Path("bench_configs")
        
        # Test matrix: (mode, size, vcpu, memory, port)
        self.test_cases = []
        modes = ["safe-single", "safe-batch", "fast-single", "fast-batch"]
        sizes = [
            ("small", "0.5", "250M", 8081),
            ("medium", "1.0", "250M", 8082),
            ("large", "1.0", "1G", 8083),
            ("xlarge", "2.0", "2G", 8084)
        ]
        
        port = 8081
        for mode in modes:
            for size_name, vcpu, mem, _ in sizes:
                self.test_cases.append((mode, size_name, vcpu, mem, port))
                port += 1
        
        self.all_results = []
    
    def setup(self):
        """Setup directories"""
        print("="*60)
        print("SNDV-KV Docker Benchmark System")
        print("="*60)
        print(f"\nRun ID: {self.run_id}")
        print(f"Test Cases: {len(self.test_cases)}")
        
        # Create directories
        for mode, size, _, _, _ in self.test_cases:
            case_dir = self.results_dir / f"{mode}-{size}"
            (case_dir / "data").mkdir(parents=True, exist_ok=True)
            (case_dir / "logs").mkdir(parents=True, exist_ok=True)
            (case_dir / "artifacts").mkdir(parents=True, exist_ok=True)
        
        # Generate configs
        self.generate_configs()
        print("✅ Setup complete\n")
    
    def generate_configs(self):
        """Generate config files"""
        self.configs_dir.mkdir(exist_ok=True)
        
        base_config = {
            "port": 8080,
            "data_dir": "/app/data",
            "wal_path": "/app/data/wal.log",
            "log_dir": "/app/logs",
            "max_memtable_size": 64 * 1024 * 1024,
            "l0_compaction_trigger": 4,
            "key_cache_size": 40000,
            "log_level": "ERROR"
        }
        
        configs = {
            "safe-single": {**base_config, "durability": True},
            "safe-batch": {**base_config, "durability": True},
            "fast-single": {**base_config, "durability": False},
            "fast-batch": {**base_config, "durability": False}
        }
        
        for mode, config in configs.items():
            with open(self.configs_dir / f"{mode}.json", "w") as f:
                json.dump(config, f, indent=2)
    
    def build_image(self):
        """Build Docker image"""
        print("Building Docker image...")
        
        result = subprocess.run(
            ["docker", "build", "-f", "Dockerfile.benchmark", 
             "-t", "sndv-kv:benchmark", "."],
            capture_output=True,
            text=True
        )
        
        if result.returncode != 0:
            print(f"❌ Build failed:\n{result.stderr}")
            return False
        
        print("✅ Image built\n")
        return True
    
    def run_test_case(self, mode, size, vcpu, mem, port):
        """Run single test case"""
        case_name = f"{mode}-{size}"
        print(f"[{self.test_cases.index((mode, size, vcpu, mem, port)) + 1}/{len(self.test_cases)}] {case_name} ({vcpu} vCPU, {mem})")
        
        case_dir = self.results_dir / case_name
        container_name = f"bench-{case_name}-{self.run_id}"
        
        # Start container
        cmd = [
            "docker", "run", "--name", container_name, "--rm", "-d",
            "--cpus", vcpu, "--memory", mem,
            "-v", f"{case_dir.absolute()}/data:/app/data",
            "-v", f"{case_dir.absolute()}/logs:/app/logs",
            "-v", f"{self.configs_dir.absolute()}/{mode}.json:/config/config.json:ro",
            "-v", f"{case_dir.absolute()}/artifacts:/artifacts",
            "-e", f"TEST_CASE={case_name}",
            "-e", f"VCPU={vcpu}",
            "-e", f"MEMORY={mem}",
            "-p", f"{port}:8080",
            "sndv-kv:benchmark",
            "-config", "/config/config.json"
        ]
        
        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0:
            print(f"  ❌ Start failed")
            return None
        
        time.sleep(3)  # Wait for server start
        
        # Run tests
        test_result = subprocess.run(
            ["python3", "scripts/docker_integration.py",
             f"http://localhost:{port}",
             str(case_dir.absolute() / "artifacts")],
            capture_output=True,
            text=True
        )
        
        # Stop container
        subprocess.run(["docker", "stop", container_name], capture_output=True)
        
        # Load results
        results_file = case_dir / "artifacts" / "integration_results.json"
        if results_file.exists():
            with open(results_file) as f:
                results = json.load(f)
                self.all_results.append(results)
                print(f"  ✅ Complete")
                return results
        
        print(f"  ❌ No results")
        return None
    
    def run_all(self):
        """Run all test cases"""
        print("Running benchmark matrix...\n")
        
        for mode, size, vcpu, mem, port in self.test_cases:
            self.run_test_case(mode, size, vcpu, mem, port)
            
            # Clean data between runs
            case_dir = self.results_dir / f"{mode}-{size}"
            shutil.rmtree(case_dir / "data", ignore_errors=True)
            (case_dir / "data").mkdir(exist_ok=True)
    
    def generate_report(self):
        """Generate comprehensive report"""
        print("\nGenerating report...")
        
        report = self.results_dir / "COMPARATIVE_REPORT.md"
        
        with open(report, "w") as f:
            f.write(f"# SNDV-KV Docker Benchmark Report\n\n")
            f.write(f"**Run ID:** {self.run_id}\n")
            f.write(f"**Date:** {datetime.now().isoformat()}\n")
            f.write(f"**Test Cases:** {len(self.test_cases)}\n\n")
            
            f.write("## Results Summary\n\n")
            f.write("| Test Case | vCPU | Memory | Single TPS | Batch TPS | p95 Latency |\n")
            f.write("|-----------|------|--------|------------|-----------|-------------|\n")
            
            for r in self.all_results:
                case = r["test_case"]
                vcpu = r["vcpu"]
                mem = r["memory"]
                single = r["tests"].get("benchmark_single", {}).get("tps", 0)
                batch = r["tests"].get("benchmark_batch", {}).get("tps", 0)
                p95 = r["tests"].get("benchmark_single", {}).get("p95_ms", 0)
                
                f.write(f"| {case} | {vcpu} | {mem} | {single:,.0f} | {batch:,.0f} | {p95:.2f}ms |\n")
            
            f.write("\n## Best Configurations\n\n")
            
            # Find best
            if self.all_results:
                best_single = max(self.all_results, 
                    key=lambda r: r["tests"].get("benchmark_single", {}).get("tps", 0))
                best_batch = max(self.all_results,
                    key=lambda r: r["tests"].get("benchmark_batch", {}).get("tps", 0))
                
                f.write(f"**Best Single TPS:** {best_single['test_case']} - ")
                f.write(f"{best_single['tests']['benchmark_single']['tps']:,.0f} TPS\n\n")
                
                f.write(f"**Best Batch TPS:** {best_batch['test_case']} - ")
                f.write(f"{best_batch['tests']['benchmark_batch']['tps']:,.0f} TPS\n")
        
        print(f"✅ Report: {report}")

def main():
    orch = BenchOrchestrator()
    orch.setup()
    
    if not orch.build_image():
        return 1
    
    orch.run_all()
    orch.generate_report()
    
    print("\n" + "="*60)
    print(f"✅ Benchmark complete!")
    print(f"Results: {orch.results_dir}")
    print("="*60)

if __name__ == "__main__":
    main()