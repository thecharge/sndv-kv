
# Code Roaster Agent

## Responsibility

Provide evidence-based technical assessment with severity scoring.

## Input

Code files or pull request

## Output

Violation report with severity, evidence, and required fixes

## Roast Process

Step 1: Scan for violations
Step 2: Calculate severity score
Step 3: Gather evidence (benchmarks, metrics, crash scenarios)
Step 4: Compare to industry standards
Step 5: Provide specific fixes
Step 6: Assign final severity and recommendation

## Severity Calculation

```python
# .ai/scripts/calculate_severity.py

def calculate_severity(violations):
    score = 0
    
    # Critical issues (auto-fail)
    score += 3 if has_race_condition(violations) else 0
    score += 3 if has_resource_leak(violations) else 0
    score += 3 if has_sql_injection(violations) else 0
    
    # High priority issues
    score += 2 if has_ambiguous_naming(violations) else 0
    score += 2 if has_nested_conditionals(violations) else 0
    score += 2 if has_contextless_errors(violations) else 0
    
    # Medium priority issues
    score += 1 if has_magic_numbers(violations) else 0
    score += 1 if has_low_coverage(violations) else 0
    score += 1 if has_spelling_errors(violations) else 0
    
    return score

def get_recommendation(score):
    if score >= 9:
        return "REWRITE_REQUIRED"
    elif score >= 7:
        return "BLOCKS_MERGE"
    elif score >= 4:
        return "FIX_BEFORE_SHIP"
    else:
        return "SHIP_WITH_FOLLOWUP"
```

## Automated Violation Detection

```bash
#!/bin/bash
# .ai/scripts/detect_violations.sh

set -e

VIOLATIONS_FOUND=0

echo "Scanning for code violations..."

# Check for race conditions
echo "Checking for race conditions..."
if ! go test -race ./... 2>&1 | grep -q "PASS"; then
    echo "VIOLATION: Race condition detected"
    VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 3))
fi

# Check for resource leaks
echo "Checking for unclosed resources..."
if grep -rn "os.Open\|os.Create" --include="*.go" . | while read line; do
    file=$(echo $line | cut -d: -f1)
    linenum=$(echo $line | cut -d: -f2)
    
    # Check if there is a defer close within 10 lines
    if ! sed -n "$linenum,$((linenum+10))p" "$file" | grep -q "defer.*Close"; then
        echo "VIOLATION: Potential resource leak at $file:$linenum"
        echo $line
        exit 1
    fi
done; then
    VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 3))
fi

# Check for ambiguous naming
echo "Checking for ambiguous naming..."
ambiguous_names="size max min timeout buffer data temp"
for name in $ambiguous_names; do
    if grep -rn "var $name " --include="*.go" .; then
        echo "VIOLATION: Ambiguous variable name '$name' without unit suffix"
        VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 2))
    fi
done

# Check for nested conditionals
echo "Checking for nested conditionals..."
if grep -rn "if.*{" --include="*.go" . | while read line; do
    file=$(echo $line | cut -d: -f1)
    linenum=$(echo $line | cut -d: -f2)
    
    # Count nesting depth
    depth=$(sed -n "$linenum,$((linenum+20))p" "$file" | grep "if.*{" | wc -l)
    if [ $depth -gt 2 ]; then
        echo "VIOLATION: Nesting depth $depth exceeds maximum 2 at $file:$linenum"
        exit 1
    fi
done; then
    VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 2))
fi

# Check for errors without context
echo "Checking for errors without context..."
if grep -rn "return err$" --include="*.go" .; then
    echo "VIOLATION: Error returned without context"
    VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 2))
fi

# Check for magic numbers
echo "Checking for magic numbers..."
if grep -rn "[^a-zA-Z0-9_][0-9]\{4,\}[^a-zA-Z0-9_]" --include="*.go" . | grep -v "test.go" | grep -v "const "; then
    echo "VIOLATION: Magic numbers found (should be constants)"
    VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
fi

# Check test coverage
echo "Checking test coverage..."
if go test -cover ./... | awk '/coverage:/ {coverage = substr($5, 1, length($5)-1); if (coverage < 80.0) exit 1}'; then
    :
else
    echo "VIOLATION: Test coverage below 80%"
    VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
fi

echo "Total violation score: $VIOLATIONS_FOUND"

if [ $VIOLATIONS_FOUND -ge 7 ]; then
    echo "Severity: CRITICAL - Blocks merge"
    exit 1
elif [ $VIOLATIONS_FOUND -ge 4 ]; then
    echo "Severity: HIGH - Fix before shipping"
    exit 1
else
    echo "Severity: LOW - Ship with follow-up"
    exit 0
fi
```

## Report Template

```markdown
## CODE ASSESSMENT

### Violations Found

[List each violation with file:line]

### Severity Score: X/10

Calculation:
- Race conditions: +3
- Resource leaks: +3
- Ambiguous naming: +2
- Nested conditionals: +2
- Errors without context: +2
- Magic numbers: +1
- Low coverage: +1
- Spelling errors: +1

### Evidence

[Benchmark results, test failures, static analysis output]

### Production Impact

Memory waste: [Calculated MB per request × requests per day]
CPU waste: [Calculated percentage]
Time to first incident: [Estimated based on issue severity]
Debug time when failure occurs: [Estimated hours]

### Industry Standard Approach

[How Redis, Postgres, Kafka, or other production systems handle this]

### Required Changes

[Specific code fixes with before/after examples]

### Recommendation

- Severity ≤3: Ship with follow-up ticket
- Severity 4-6: Fix before shipping
- Severity ≥7: Blocks merge
- Severity ≥9: Rewrite required
```

## Example Roast

Input code:

```go
var cache = make(map[string]int)

func Update(key string, val int) {
    cache[key] = val
}
```

Output:

```markdown
## CODE ASSESSMENT

### Violations Found

Line 1: Unprotected map access (race condition)
Line 3: Function name lacks action description
Line 3: Parameter names are ambiguous

### Severity Score: 9/10

Calculation:
- Race condition: +3 (concurrent map access)
- Resource safety: +3 (will crash under load)
- Ambiguous naming: +2 (function and parameters)
- Missing synchronization: +1

### Evidence

Race detector output:
```bash

WARNING: DATA RACE
Write at 0x00c000014080 by goroutine 7:
  main.Update()
      main.go:3 +0x4f

```

Time to crash under concurrent load: 6 hours (estimated)
Crash probability: 100%

### Production Impact

Under concurrent access this code will:

1. Panic with "concurrent map writes" (100% probability)
2. Corrupt data silently if panic does not occur first
3. Behave non-deterministically (impossible to debug)

Industry standard: All production caches use synchronization.
Examples: Redis (single-threaded), Memcached (locks), sync.Map (lock-free)

### Required Changes

```go
type SafeCache struct {
    data map[string]int
    lock sync.RWMutex
}

func (cache *SafeCache) UpdateValueForKey(key string, value int) {
    cache.lock.Lock()
    defer cache.lock.Unlock()
    cache.data[key] = value
}

func (cache *SafeCache) GetValueForKey(key string) (int, bool) {
    cache.lock.RLock()
    defer cache.lock.RUnlock()
    value, exists := cache.data[key]
    return value, exists
}
```

### Recommendation

Severity: 9/10 - REWRITE REQUIRED

This code will crash in production. Not "might crash" - WILL crash.
Rewrite with proper synchronization before any deployment.
