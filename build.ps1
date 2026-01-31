param (
    [string]$Target = "build"
)

$Binary = "sndv-kv.exe"
$Pkg = "./cmd/server/main.go"

Switch ($Target) {
    "build" {
        Write-Host "🔨 Building Optimized Binary..." -ForegroundColor Cyan
        go build -ldflags="-s -w" -o $Binary $Pkg
        Write-Host "✅ Done." -ForegroundColor Green
    }

    "test" {
        Write-Host "🧪 Running Unit Tests..." -ForegroundColor Cyan
        go test ./internal/... -v
    }

    "coverage" {
        Write-Host "📊 Checking Coverage..." -ForegroundColor Cyan
        go test ./internal/... -coverprofile=coverage.out
        go tool cover -func=coverage.out
        Remove-Item coverage.out
    }

    "bench" {
        Write-Host "🚀 Running 100k Benchmark..." -ForegroundColor Cyan
        # Ensure fast config exists
        $conf = @{
            port=9092; data_dir="./data_bench"; wal_path="./data_bench/wal.log";
            durability=$false; auth_token=""; max_memtable_size=67108864
        }
        $conf | ConvertTo-Json | Out-File "config_fast.json" -Encoding ASCII

        python scripts/benchmark_100k.py
    }

    "clean" {
        Write-Host "🧹 Cleaning up..." -ForegroundColor Yellow
        if (Test-Path $Binary) { Remove-Item $Binary }
        if (Test-Path "./data") { Remove-Item "./data" -Recurse -Force }
        if (Test-Path "./data_bench") { Remove-Item "./data_bench" -Recurse -Force }
        if (Test-Path "./data_safe") { Remove-Item "./data_safe" -Recurse -Force }
        if (Test-Path "./data_fast") { Remove-Item "./data_fast" -Recurse -Force }
        Write-Host "✨ Clean."
    }

    Default {
        Write-Host "Usage: .\build.ps1 [build|test|coverage|bench|clean]" -ForegroundColor Red
    }
}