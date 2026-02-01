#!/usr/bin/env python3
"""
SNDV-KV Comprehensive Benchmark Orchestrator
Real metrics: CPU, memory, disk usage, WAL size, data size
Docker-based with proper artifact extraction
"""

import subprocess
import time
import json
import shutil
import requests
import statistics
import os
from pathlib import Path
from datetime import datetime

class ComprehensiveBenchOrchestrator:
    def __init__(self):
        self.run_id = datetime.now().strftime("%Y%m%d_%H%M%S")
        self.results_dir = Path(f"benchmark_results_{self.run_id}")
        self.configs_dir = Path("bench_configs")
        
        # REAL test matrix with actual resource constraints
        # Format: (mode, vcpu, memory, port, expected_load_type)
        self.test_cases = [
            # Low resources - see how it behaves under constraint
            ("safe-single", "0.5", "256M", 8081, "light"),
            ("safe-batch", "0.5", "256M", 8082, "light"),
            ("fast-single", "0.5", "256M", 8083, "light"),
            ("fast-batch", "0.5", "256M", 8084, "light"),
            
            # Medium resources - 1 vCPU, memory scaling
            ("safe-single", "1.0", "256M", 8085, "medium"),
            ("safe-single", "1.0", "512M", 8086, "medium"),
            ("safe-single", "1.0", "1024M", 8087, "medium"),
            ("safe-batch", "1.0", "256M", 8088, "medium"),
            ("safe-batch", "1.0", "512M", 8089, "medium"),
            ("safe-batch", "1.0", "1024M", 8090, "medium"),
            
            # High resources - 2 vCPU, high memory
            ("safe-single", "2.0", "1024M", 8091, "high"),
            ("safe-single", "2.0", "2048M", 8092, "high"),
            ("safe-batch", "2.0", "1024M", 8093, "high"),
            ("safe-batch", "2.0", "2048M", 8094, "high"),
            ("fast-single", "2.0", "2048M", 8095, "high"),
            ("fast-batch", "2.0", "2048M", 8096, "high"),
        ]
        
        self.all_results = []
    
    def setup(self):
        """Setup directories and extract build artifacts"""
        print("="*80)
        print("SNDV-KV COMPREHENSIVE BENCHMARK")
        print("="*80)
        print(f"\nRun ID: {self.run_id}")
        print(f"Test Cases: {len(self.test_cases)}")
        print(f"Output: {self.results_dir}\n")
        
        # Create base directories
        self.results_dir.mkdir(exist_ok=True)
        (self.results_dir / "build_artifacts").mkdir(exist_ok=True)
        
        # Generate all configs
        self.generate_all_configs()
        print("✅ Configs generated\n")
    
    def generate_all_configs(self):
        """Generate all configuration files"""
        self.configs_dir.mkdir(exist_ok=True)
        
        # Base config
        base = {
            "server_port": 8080,
            "data_directory_path": "/app/data",
            "write_ahead_log_file_path": "/app/data/wal.log",
            "log_directory_path": "/app/logs",
            "maximum_memtable_size_in_bytes": 67108864,  # 64MB
            "level_zero_compaction_trigger_count": 4,
            "sstable_block_size_in_bytes": 4096,
            "bloom_filter_false_positive_rate": 0.01,
            "compaction_interval_in_seconds": 5,
            "authentication_secret": "BENCHMARK_SECRET",
            "authentication_token": "",  # Disabled for bench
            "maximum_cpu_count": 0,
            "maximum_system_memory_in_bytes": 0,
            "enable_pprof_profiling": False,
            "key_cache_capacity_count": 40000,
            "log_severity_level": "INFO"
        }
        
        # Safe configs (durability ON)
        safe_single = {**base, "enable_disk_durability": True}
        safe_batch = {**base, "enable_disk_durability": True}
        
        # Fast configs (durability OFF)
        fast_single = {**base, "enable_disk_durability": False}
        fast_batch = {**base, "enable_disk_durability": False}
        
        configs = {
            "safe-single": safe_single,
            "safe-batch": safe_batch,
            "fast-single": fast_single,
            "fast-batch": fast_batch,
        }
        
        for name, config in configs.items():
            with open(self.configs_dir / f"{name}.json", 'w') as f:
                json.dump(config, f, indent=2)
    
    def build_image(self):
        """Build Docker image"""
        print("Building Docker image (multi-stage)...")
        print("  Stage 1: Unit tests + coverage")
        print("  Stage 2: Go benchmarks + profiling")
        print("  Stage 3: Binary build + metrics")
        print("  Stage 4: Runtime image")
        print()
        
        result = subprocess.run(
            ["docker", "build", "-f", "Dockerfile.benchmark", "-t", "sndv-kv:benchmark", "."],
            capture_output=True,
            text=True
        )
        
        if result.returncode != 0:
            print(f"❌ Build failed:\n{result.stderr}")
            return False
        
        print("✅ Image built\n")
        
        # Extract build artifacts ONCE
        print("📦 Extracting build artifacts...")
        self.extract_build_artifacts()
        
        return True
    
    def extract_build_artifacts(self):
        """Extract build artifacts from image"""
        temp_container = f"extract-{self.run_id}"
        artifacts_dir = self.results_dir / "build_artifacts"
        
        try:
            # Create temp container
            subprocess.run(
                ["docker", "create", "--name", temp_container, "sndv-kv:benchmark"],
                capture_output=True,
                check=True
            )
            
            # Copy artifacts
            subprocess.run(
                ["docker", "cp", f"{temp_container}:/artifacts/.", str(artifacts_dir)],
                capture_output=True,
                check=True
            )
            
            print(f"✅ Extracted to {artifacts_dir}")
            
            # List what we got
            if (artifacts_dir / "unit-tests").exists():
                print("   - Unit test results + coverage")
            if (artifacts_dir / "benchmarks").exists():
                print("   - Go benchmark results + profiles")
            if (artifacts_dir / "build").exists():
                print("   - Build metrics")
            
        except Exception as e:
            print(f"⚠️  Could not extract: {e}")
        finally:
            subprocess.run(["docker", "rm", temp_container], capture_output=True)
    
    def run_test_case(self, mode, vcpu, memory, port, load_type):
        """Run single test case with full metrics collection"""
        case_name = f"{mode}-{vcpu}cpu-{memory}"
        idx = self.test_cases.index((mode, vcpu, memory, port, load_type)) + 1
        
        print(f"\n[{idx}/{len(self.test_cases)}] {case_name}")
        print("="*80)
        print(f"  Resources: {vcpu} vCPU, {memory} RAM")
        print(f"  Mode: {mode}")
        print(f"  Expected load: {load_type}")
        print("-"*80)
        
        # Create case directory
        case_dir = self.results_dir / case_name
        case_dir.mkdir(exist_ok=True)
        (case_dir / "data").mkdir(exist_ok=True)
        (case_dir / "logs").mkdir(exist_ok=True)
        (case_dir / "metrics").mkdir(exist_ok=True)
        
        # Start container
        container_name = f"bench-{case_name}-{self.run_id}"
        
        cmd = [
            "docker", "run",
            "--name", container_name,
            "--rm", "-d",
            "--cpus", vcpu,
            "--memory", memory,
            "-v", f"{case_dir.absolute() / 'data'}:/app/data",
            "-v", f"{case_dir.absolute() / 'logs'}:/app/logs",
            "-v", f"{self.configs_dir.absolute() / f'{mode}.json'}:/config/config.json:ro",
            "-p", f"{port}:8080",
            "sndv-kv:benchmark",
            "-config", "/config/config.json"
        ]
        
        print("  🚀 Starting container...", end=" ", flush=True)
        result = subprocess.run(cmd, capture_output=True, text=True)
        
        if result.returncode != 0:
            print(f"❌\n     {result.stderr}")
            return None
        
        container_id = result.stdout.strip()
        print(f"✅ ({container_id[:12]})")
        
        # Wait for server ready
        server_url = f"http://localhost:{port}"
        if not self.wait_for_server(server_url, timeout=15):
            print("  ❌ Server timeout")
            subprocess.run(["docker", "stop", container_name], capture_output=True)
            return None
        
        print("  ✅ Server ready")
        
        try:
            # Run benchmarks with monitoring
            results = self.run_benchmarks_with_monitoring(
                server_url, container_name, case_dir, mode, load_type
            )
            
            # Collect resource usage
            # self.collect_resource_metrics(container_name, case_dir)
            
            # Collect disk usage
            # self.collect_disk_metrics(case_dir)
            
            # Save results
            results['test_case'] = case_name
            results['vcpu'] = vcpu
            results['memory'] = memory
            results['mode'] = mode
            results['load_type'] = load_type
            results['timestamp'] = datetime.now().isoformat()
            results['resources'] = self.collect_resource_metrics(container_name, case_dir)
            results['disk'] = self.collect_disk_metrics(case_dir)

            
            with open(case_dir / "metrics" / "results.json", 'w') as f:
                json.dump(results, f, indent=2)
            
            self.all_results.append(results)
            
            # Print summary
            print("\n  📊 Results:")
            print(f"     Single TPS:     {results['single']['tps']:>10,.0f}")
            print(f"     Batch TPS:      {results['batch']['tps']:>10,.0f}")
            print(f"     Memory Used:    {results['resources']['memory_mb']:>10.2f} MB")
            print(f"     CPU Used:       {results['resources']['cpu_percent']:>10.2f}%")
            print(f"     Data Size:      {results['disk']['data_mb']:>10.2f} MB")
            print(f"     WAL Size:       {results['disk']['wal_mb']:>10.2f} MB")
            print(f"  ✅ Complete")
            
            return results
            
        finally:
            # Stop container
            subprocess.run(["docker", "stop", container_name], capture_output=True)
            time.sleep(2)  # Cooldown
    
    def wait_for_server(self, url, timeout=15):
        """Wait for server to be ready"""
        start = time.time()
        while time.time() - start < timeout:
            try:
                resp = requests.get(f"{url}/metrics", timeout=1)
                if resp.status_code == 200:
                    return True
            except:
                pass
            time.sleep(0.5)
        return False
    
    def run_benchmarks_with_monitoring(self, server_url, container_name, case_dir, mode, load_type):
        """Run benchmarks and collect metrics"""
        results = {
            'warmup': {},
            'single': {},
            'batch': {},
            'stress': {},
            'resources': {},
            'disk': {}
        }
        
        # Warmup
        print("  🔥 Warmup (100 ops)...", end=" ", flush=True)
        for i in range(100):
            try:
                requests.post(f"{server_url}/put",
                    json={"key": f"w{i}", "value": "x"*64, "ttl": 0},
                    timeout=2)
            except:
                pass
        print("✅")
        
        # Single operation benchmark
        print("  📝 Single operations (1000)...", end=" ", flush=True)
        results['single'] = self.benchmark_single(server_url, 1000)
        print(f"{results['single']['tps']:,.0f} TPS (p95: {results['single']['p95_ms']:.2f}ms)")
        
        # Batch operation benchmark
        print("  📦 Batch operations (5000 items)...", end=" ", flush=True)
        results['batch'] = self.benchmark_batch(server_url, 5000, 100)
        print(f"{results['batch']['tps']:,.0f} TPS")
        
        # Stress test (based on load type)
        if load_type in ['medium', 'high']:
            stress_ops = 10000 if load_type == 'high' else 5000
            print(f"  💥 Stress test ({stress_ops} ops)...", end=" ", flush=True)
            results['stress'] = self.benchmark_stress(server_url, stress_ops)
            print(f"{results['stress']['tps']:,.0f} TPS")
        
        return results
    
    def benchmark_single(self, server_url, count):
        """Benchmark single operations"""
        latencies = []
        success = 0
        
        start = time.time()
        for i in range(count):
            t0 = time.perf_counter()
            try:
                resp = requests.post(f"{server_url}/put",
                    json={"key": f"single_{i}", "value": "x"*128, "ttl": 0},
                    timeout=3)
                lat = (time.perf_counter() - t0) * 1000
                latencies.append(lat)
                if resp.status_code == 201:
                    success += 1
            except:
                latencies.append(3000)
        
        duration = time.time() - start
        latencies.sort()
        
        return {
            'count': count,
            'success': success,
            'tps': success / duration if duration > 0 else 0,
            'p50_ms': latencies[len(latencies)//2] if latencies else 0,
            'p95_ms': latencies[int(len(latencies)*0.95)] if latencies else 0,
            'p99_ms': latencies[int(len(latencies)*0.99)] if latencies else 0,
            'avg_ms': statistics.mean(latencies) if latencies else 0
        }
    
    def benchmark_batch(self, server_url, total, batch_size):
        """Benchmark batch operations"""
        success = 0
        num_batches = total // batch_size
        
        start = time.time()
        for i in range(num_batches):
            items = [{"key": f"batch_{i}_{j}", "value": "x"*128, "ttl": 0}
                     for j in range(batch_size)]
            try:
                resp = requests.post(f"{server_url}/batch",
                    json={"items": items},
                    timeout=10)
                if resp.status_code == 201:
                    success += batch_size
            except:
                pass
        
        duration = time.time() - start
        
        return {
            'total': total,
            'batch_size': batch_size,
            'success': success,
            'tps': success / duration if duration > 0 else 0,
            'batches': num_batches
        }
    
    def benchmark_stress(self, server_url, count):
        """Stress test with mixed operations"""
        success = 0
        
        start = time.time()
        for i in range(count):
            op = i % 3
            try:
                if op == 0:  # Write
                    resp = requests.post(f"{server_url}/put",
                        json={"key": f"stress_{i}", "value": "x"*256, "ttl": 0},
                        timeout=3)
                elif op == 1:  # Read
                    resp = requests.get(f"{server_url}/get?key=stress_{i-100}",
                        timeout=3)
                else:  # Batch
                    items = [{"key": f"stress_b_{i}_{j}", "value": "x", "ttl": 0}
                             for j in range(10)]
                    resp = requests.post(f"{server_url}/batch",
                        json={"items": items},
                        timeout=5)
                
                if resp.status_code in [200, 201]:
                    success += 1
            except:
                pass
        
        duration = time.time() - start
        
        return {
            'count': count,
            'success': success,
            'tps': success / duration if duration > 0 else 0
        }
    
    def collect_resource_metrics(self, container_name, case_dir):
        """Collect container resource usage"""
        print("  📊 Collecting resource metrics...", end=" ", flush=True)
        
        # Get stats
        result = subprocess.run(
            ["docker", "stats", container_name, "--no-stream", "--format", "{{json .}}"],
            capture_output=True,
            text=True
        )
        
        if result.returncode == 0 and result.stdout:
            try:
                stats = json.loads(result.stdout)
                
                # Parse memory
                mem_usage = stats.get('MemUsage', '0 / 0')
                mem_used_str = mem_usage.split('/')[0].strip()
                mem_value = float(mem_used_str.replace('MiB', '').replace('GiB', '').strip())
                if 'GiB' in mem_used_str:
                    mem_value *= 1024
                
                # Parse CPU
                cpu_str = stats.get('CPUPerc', '0%').replace('%', '').strip()
                cpu_value = float(cpu_str) if cpu_str else 0
                
                metrics = {
                    'memory_mb': mem_value,
                    'cpu_percent': cpu_value,
                    'raw': stats
                }
                
                with open(case_dir / "metrics" / "resources.json", 'w') as f:
                    json.dump(metrics, f, indent=2)
                
                # Store in results
                if hasattr(self, 'current_result'):
                    self.current_result['resources'] = metrics
                
                print("✅")
                return metrics
                
            except Exception as e:
                print(f"⚠️ ({e})")
        else:
            print("⚠️ (failed)")
        
        return {'memory_mb': 0, 'cpu_percent': 0}
    
    def collect_disk_metrics(self, case_dir):
        """Collect disk usage metrics"""
        print("  💾 Collecting disk metrics...", end=" ", flush=True)
        
        data_dir = case_dir / "data"
        
        # Calculate sizes
        data_size = sum(f.stat().st_size for f in data_dir.rglob('*') if f.is_file())
        
        wal_size = 0
        wal_files = list(data_dir.glob('wal.log*'))
        for wal in wal_files:
            if wal.is_file():
                wal_size += wal.stat().st_size
        
        sstable_size = 0
        sstable_files = list(data_dir.rglob('*.sst'))
        for sst in sstable_files:
            if sst.is_file():
                sstable_size += sst.stat().st_size
        
        metrics = {
            'data_mb': data_size / (1024 * 1024),
            'wal_mb': wal_size / (1024 * 1024),
            'sstable_mb': sstable_size / (1024 * 1024),
            'wal_files': len(wal_files),
            'sstable_files': len(sstable_files),
            'total_files': len(list(data_dir.rglob('*')))
        }
        
        with open(case_dir / "metrics" / "disk.json", 'w') as f:
            json.dump(metrics, f, indent=2)
        
        print("✅")
        return metrics
    
    def run_all(self):
        """Run all test cases"""
        print("\nRunning benchmark matrix...\n")
        
        for mode, vcpu, memory, port, load_type in self.test_cases:
            result = self.run_test_case(mode, vcpu, memory, port, load_type)
            
            if result:
                # Store resource and disk metrics
                resources = self.collect_resource_metrics(
                    f"bench-{mode}-{vcpu}cpu-{memory}-{self.run_id}",
                    self.results_dir / f"{mode}-{vcpu}cpu-{memory}"
                )
                disk = self.collect_disk_metrics(
                    self.results_dir / f"{mode}-{vcpu}cpu-{memory}"
                )
                
                result['resources'] = resources
                result['disk'] = disk
    
    def generate_comprehensive_report(self):
        """Generate comprehensive report"""
        print("\n" + "="*80)
        print("Generating comprehensive report...")
        print("="*80 + "\n")
        
        # Save all results
        with open(self.results_dir / "all_results.json", 'w') as f:
            json.dump(self.all_results, f, indent=2)
        
        # Generate markdown report
        report_path = self.results_dir / "COMPREHENSIVE_REPORT.md"
        
        with open(report_path, 'w') as f:
            f.write(f"# SNDV-KV Comprehensive Benchmark Report\n\n")
            f.write(f"**Run ID:** {self.run_id}\n")
            f.write(f"**Date:** {datetime.now().isoformat()}\n")
            f.write(f"**Test Cases:** {len(self.all_results)}\n\n")
            
            # Performance table
            f.write("## Performance Summary\n\n")
            f.write("| Test Case | vCPU | RAM | Single TPS | Batch TPS | p95 Latency | Memory Used | CPU Used |\n")
            f.write("|-----------|------|-----|------------|-----------|-------------|-------------|----------|\n")
            
            for r in self.all_results:
                f.write(f"| {r['test_case']} | {r['vcpu']} | {r['memory']} | "
                       f"{r['single']['tps']:,.0f} | {r['batch']['tps']:,.0f} | "
                       f"{r['single']['p95_ms']:.2f}ms | "
                       f"{r['resources']['memory_mb']:.1f}MB | "
                       f"{r['resources']['cpu_percent']:.1f}% |\n")
            
            # Disk usage table
            f.write("\n## Disk Usage\n\n")
            f.write("| Test Case | Data Size | WAL Size | SSTables | WAL Files |\n")
            f.write("|-----------|-----------|----------|----------|----------|\n")
            
            for r in self.all_results:
                f.write(f"| {r['test_case']} | "
                       f"{r['disk']['data_mb']:.2f}MB | "
                       f"{r['disk']['wal_mb']:.2f}MB | "
                       f"{r['disk']['sstable_mb']:.2f}MB | "
                       f"{r['disk']['wal_files']} |\n")
            
            # Best configurations
            f.write("\n## Best Configurations\n\n")
            
            best_single = max(self.all_results, key=lambda r: r['single']['tps'])
            best_batch = max(self.all_results, key=lambda r: r['batch']['tps'])
            
            f.write(f"**Best Single TPS:** {best_single['test_case']}\n")
            f.write(f"- TPS: {best_single['single']['tps']:,.0f}\n")
            f.write(f"- p95: {best_single['single']['p95_ms']:.2f}ms\n")
            f.write(f"- Memory: {best_single['resources']['memory_mb']:.1f}MB\n\n")
            
            f.write(f"**Best Batch TPS:** {best_batch['test_case']}\n")
            f.write(f"- TPS: {best_batch['batch']['tps']:,.0f}\n")
            f.write(f"- Memory: {best_batch['resources']['memory_mb']:.1f}MB\n\n")
        
        print(f"✅ Report saved: {report_path}")
        print(f"✅ JSON saved: {self.results_dir / 'all_results.json'}")
        print(f"✅ Build artifacts: {self.results_dir / 'build_artifacts'}")

def main():
    orch = ComprehensiveBenchOrchestrator()
    orch.setup()
    
    if not orch.build_image():
        return 1
    
    orch.run_all()
    orch.generate_comprehensive_report()
    
    print("\n✅ Benchmark complete!")
    return 0

if __name__ == "__main__":
    import sys
    sys.exit(main())