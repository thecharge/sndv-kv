#!/usr/bin/env python3
import json
from pathlib import Path
from datetime import datetime

def generate_dashboard():
    # Load all results
    results = []
    for file in sorted(Path("benchmark_reports").glob("*.json")):
        try:
            with open(file) as f:
                results.append(json.load(f))
        except:
            pass
    
    if not results:
        print("No benchmark results found")
        return
    
    results.sort(key=lambda x: x.get('timestamp', ''))
    latest = results[-1]
    
    # Generate HTML
    html = f"""<!DOCTYPE html>
<html>
<head>
    <title>SNDV-KV Dashboard</title>
    <script src="https://cdn.plot.ly/plotly-latest.min.js"></script>
    <style>
        body {{ font-family: Arial; margin: 20px; }}
        .metric-card {{
            display: inline-block;
            margin: 10px;
            padding: 20px;
            border: 2px solid #ddd;
            border-radius: 8px;
            min-width: 200px;
        }}
        .metric-value {{ font-size: 28px; font-weight: bold; color: #2196F3; }}
        .metric-label {{ color: #666; margin-top: 5px; }}
        .chart {{ margin: 30px 0; }}
    </style>
</head>
<body>
    <h1>SNDV-KV Performance Dashboard</h1>
    <p>Last updated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>
    
    <div>
        <div class="metric-card">
            <div class="metric-value">{latest['version'][:15]}</div>
            <div class="metric-label">Latest Version</div>
        </div>
        <div class="metric-card">
            <div class="metric-value">{latest['metrics'].get('write_ops_per_sec', 0):,}</div>
            <div class="metric-label">Ops/sec</div>
        </div>
        <div class="metric-card">
            <div class="metric-value">{latest['metrics'].get('test_coverage', 'N/A')}</div>
            <div class="metric-label">Test Coverage</div>
        </div>
    </div>
    
    <div id="perf-chart" class="chart"></div>
    <div id="latency-chart" class="chart"></div>
    
    <script>
        var perfData = {{
            x: {[r['timestamp'] for r in results]},
            y: {[r['metrics'].get('write_ops_per_sec', 0) for r in results]},
            type: 'scatter',
            mode: 'lines+markers',
            name: 'Write Ops/sec',
            line: {{ color: '#2196F3' }}
        }};
        
        Plotly.newPlot('perf-chart', [perfData], {{
            title: 'Write Performance Over Time',
            xaxis: {{ title: 'Date' }},
            yaxis: {{ title: 'Operations/sec' }}
        }});
        
        var latencyData = {{
            x: {[r['timestamp'] for r in results]},
            y: {[r['metrics'].get('write_ns_per_op', 0) / 1000 for r in results]},
            type: 'scatter',
            mode: 'lines+markers',
            name: 'Latency',
            line: {{ color: '#FF5722' }}
        }};
        
        Plotly.newPlot('latency-chart', [latencyData], {{
            title: 'Latency Over Time',
            xaxis: {{ title: 'Date' }},
            yaxis: {{ title: 'Microseconds/op' }}
        }});
    </script>
</body>
</html>"""
    
    output_file = Path("benchmark_reports/dashboard.html")
    output_file.write_text(html)
    
    print(f"✅ Dashboard generated: {output_file}")
    print(f"   Open in browser to view")

if __name__ == "__main__":
    generate_dashboard()