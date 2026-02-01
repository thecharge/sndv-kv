# Pattern Scout Agent

## Responsibility

Detect duplicate patterns and find superior cross-domain solutions.

## Input

Codebase after any feature implementation

## Output

Pattern report with evidence for refactoring decision

## Detection Algorithm

```python
# .ai/scripts/detect_patterns.py

import os
import difflib
from pathlib import Path

def find_similar_code(directory, similarity_threshold=0.5):
    """Find code blocks with similarity above threshold."""
    
    go_files = list(Path(directory).rglob("*.go"))
    similar_pairs = []
    
    for i, file1 in enumerate(go_files):
        for file2 in go_files[i+1:]:
            content1 = file1.read_text().split('\n')
            content2 = file2.read_text().split('\n')
            
            similarity = difflib.SequenceMatcher(None, content1, content2).ratio()
            
            if similarity > similarity_threshold:
                similar_pairs.append({
                    'file1': str(file1),
                    'file2': str(file2),
                    'similarity': similarity
                })
    
    return similar_pairs

def count_pattern_occurrences(directory, pattern_signature):
    """Count how many times a pattern appears."""
    
    occurrences = []
    
    for go_file in Path(directory).rglob("*.go"):
        content = go_file.read_text()
        
        # Check for pattern signature (function signature, struct pattern, etc.)
        if pattern_signature in content:
            occurrences.append(str(go_file))
    
    return occurrences

def calculate_duplication_cost(similar_pairs):
    """Calculate total lines of duplicated code."""
    
    total_duplicated_lines = 0
    
    for pair in similar_pairs:
        file1_lines = Path(pair['file1']).read_text().count('\n')
        total_duplicated_lines += int(file1_lines * pair['similarity'])
    
    return total_duplicated_lines

if __name__ == "__main__":
    import sys
    
    if len(sys.argv) < 2:
        print("Usage: python detect_patterns.py <directory>")
        sys.exit(1)
    
    directory = sys.argv[1]
    
    print(f"Scanning {directory} for duplicate patterns...")
    
    similar_pairs = find_similar_code(directory, similarity_threshold=0.5)
    
    if similar_pairs:
        print(f"\nFound {len(similar_pairs)} similar code pairs:")
        for pair in similar_pairs:
            print(f"  {pair['file1']} <-> {pair['file2']} ({pair['similarity']*100:.1f}% similar)")
        
        duplicated_lines = calculate_duplication_cost(similar_pairs)
        print(f"\nTotal duplicated code: approximately {duplicated_lines} lines")
        print("\nRecommendation: Consider extracting common pattern to base implementation")
    else:
        print("No significant code duplication detected")
```

## Cross-Domain Pattern Library

```yaml
# .ai/config/cross_domain_patterns.yml

patterns:
  - name: "DNA Error Correction"
    domain: "Biology"
    problem: "Data corruption detection and recovery"
    solution: "Redundant encoding with error correction codes"
    implementation: "CRC32 checksums, Reed-Solomon codes, parity bits"
    example: "event_codec.go uses CRC32 for corruption detection"
    
  - name: "Ant Colony Optimization"
    domain: "Biology"
    problem: "Load balancing without central coordinator"
    solution: "Success-based routing with pheromone-like decay"
    implementation: "Weighted probability selection that decays over time"
    example: "Load balancer picks nodes based on success score"
    
  - name: "Entropy Management"
    domain: "Physics"
    problem: "Unbounded growth of logs, caches, state"
    solution: "Periodic compaction to reduce disorder"
    implementation: "Time-based or size-based cleanup routines"
    example: "WAL compaction every hour or when size exceeds limit"
    
  - name: "Merkle Trees"
    domain: "Mathematics"
    problem: "Efficient verification of large dataset integrity"
    solution: "Hash tree structure for O(log N) verification"
    implementation: "Binary tree where each node is hash of children"
    example: "Git uses Merkle trees for commit verification"
    
  - name: "Bloom Filters"
    domain: "Mathematics"
    problem: "Membership testing with memory constraints"
    solution: "Probabilistic data structure with configurable false positive rate"
    implementation: "Bit array with multiple hash functions"
    example: "Deduplication with 1% false positive, 100x memory reduction"
    
  - name: "Gossip Protocol"
    domain: "Distributed Systems"
    problem: "State synchronization without central coordination"
    solution: "Periodic random peer communication for state propagation"
    implementation: "Each node randomly picks peers and shares state"
    example: "Cassandra uses gossip for cluster membership"
```

## Decision Matrix

Recommend refactoring when meeting threshold requirements:

```python
# .ai/scripts/refactor_decision.py

def should_refactor(metrics):
    """Decide if refactoring is justified based on evidence."""
    
    conditions_met = 0
    
    # Code reduction
    if metrics['code_reduction_percent'] >= 30:
        conditions_met += 1
        print(f"✓ Code reduction: {metrics['code_reduction_percent']}% (threshold: 30%)")
    
    # Complexity reduction
    if metrics['complexity_reduction_percent'] >= 20:
        conditions_met += 1
        print(f"✓ Complexity reduction: {metrics['complexity_reduction_percent']}% (threshold: 20%)")
    
    # Performance improvement
    if metrics['performance_improvement_percent'] >= 5:
        conditions_met += 1
        print(f"✓ Performance improvement: {metrics['performance_improvement_percent']}% (threshold: 5%)")
    
    # Bug class elimination
    if metrics['bugs_eliminated'] > 0:
        conditions_met += 1
        print(f"✓ Bugs eliminated: {metrics['bugs_eliminated']}")
    
    # Coverage improvement
    if metrics['coverage_improvement_percent'] > 0:
        conditions_met += 1
        print(f"✓ Coverage improvement: {metrics['coverage_improvement_percent']}%")
    
    # Requirements
    all_tests_pass = metrics['tests_passing']
    no_regression = metrics['performance_regression_percent'] <= 5
    poc_exists = metrics['poc_implemented']
    
    print(f"\nTests passing: {all_tests_pass}")
    print(f"Performance regression: {metrics['performance_regression_percent']}% (max: 5%)")
    print(f"POC implemented: {poc_exists}")
    
    # Decision
    if conditions_met >= 2 and all_tests_pass and no_regression and poc_exists:
        return "REFACTOR"
    elif conditions_met >= 1:
        return "CONSIDER"
    else:
        return "KEEP"
```

## Pattern Report Format

```markdown
## PATTERN ANALYSIS

### Pattern Detected
[Pattern name and description]

Occurrences: [Number of similar code blocks]
Files affected: [List of files]
Total duplicated lines: [Count]
Code similarity: [Percentage]

### Cross-Domain Insight
Domain: [Biology, Physics, Math, Distributed Systems]
Inspiration: [Specific concept or algorithm]
How it solves the problem: [Explanation]

### Current Metrics
Lines of code: [Count]
Files: [Count]
Cyclomatic complexity: [Average across affected code]
Test coverage: [Percentage]
Performance: [Benchmark results]

### Proposed Metrics
Lines of code: [Count] ([X]% reduction)
Files: [Count] ([X]% consolidation)
Cyclomatic complexity: [Average] ([X]% reduction)
Test coverage: [Percentage] ([X]% improvement)
Performance: [Benchmark results] ([X]% improvement)

### Proof of Concept
Implementation: [Link to POC branch or file]
Tests: [All passing / X failing]
Benchmarks: [Comparison results]

### Decision
Conditions met: [X/5]
- Code reduction ≥30%: [Yes/No]
- Complexity reduction ≥20%: [Yes/No]
- Performance gain ≥5%: [Yes/No]
- Bugs eliminated: [Count]
- Coverage improved: [Yes/No]

Requirements:
- POC implemented: [Yes/No]
- All tests passing: [Yes/No]
- No performance regression >5%: [Yes/No]

Recommendation: [REFACTOR / CONSIDER / KEEP]
Confidence: [HIGH / MEDIUM / LOW]
```
