# Test Generator Agent

## Responsibility

Create comprehensive test suites using template pattern for maximum coverage.

## Input

Interface definition or implementation to test

## Output

Test suite with 100% interface coverage and disaster recovery tests

## Test Strategy

```bash
Interface (tested at 100%)
    ├─ Implementation A (inherits 80-90% coverage)
    ├─ Implementation B (inherits 80-90% coverage)
    └─ Implementation C (inherits 80-90% coverage)

Effort: 1 comprehensive suite + 3 small additions
Result: 80-90% coverage across all implementations
```

## Test Suite Template

```go
// [interface_name]_test_suite.go

package [package]

import "testing"

type [InterfaceName]TestSuite struct {
    Name           string
    Create[Interface] func() [InterfaceName]
    Cleanup        func()
}

func (suite *[InterfaceName]TestSuite) RunAllTests(t *testing.T) {
    if suite.Cleanup != nil {
        defer suite.Cleanup()
    }
    
    t.Run(suite.Name, func(t *testing.T) {
        // Happy path tests
        suite.testHappyPath(t)
        suite.testMultipleOperations(t)
        suite.testEdgeCases(t)
        
        // Critical path tests
        suite.testErrorHandling(t)
        suite.testConcurrency(t)
        suite.testBoundaryConditions(t)
        
        // Disaster recovery tests
        suite.testCorruptionDetection(t)
        suite.testCorruptionRecovery(t)
        suite.testResourceExhaustion(t)
    })
}

// Happy path: basic functionality works
func (suite *[InterfaceName]TestSuite) testHappyPath(t *testing.T) {
    t.Run("HappyPath", func(t *testing.T) {
        instance := suite.Create[Interface]()
        // Test basic operations
    })
}

// Critical path: handles errors correctly
func (suite *[InterfaceName]TestSuite) testErrorHandling(t *testing.T) {
    t.Run("ErrorHandling", func(t *testing.T) {
        instance := suite.Create[Interface]()
        // Test error scenarios
    })
}

// Disaster recovery: recovers from failures
func (suite *[InterfaceName]TestSuite) testCorruptionRecovery(t *testing.T) {
    t.Run("CorruptionRecovery", func(t *testing.T) {
        instance := suite.Create[Interface]()
        // Test recovery from corruption
    })
}
```

## Test Generation Script

```bash
#!/bin/bash
# .ai/scripts/generate_tests.sh

set -e

if [ $# -lt 2 ]; then
    echo "Usage: ./generate_tests.sh <package> <interface_name>"
    exit 1
fi

PACKAGE=$1
INTERFACE=$2
SUITE_FILE="${PACKAGE}/${INTERFACE}_test_suite.go"

echo "Generating test suite for ${INTERFACE} in package ${PACKAGE}..."

cat > "$SUITE_FILE" << EOF
package ${PACKAGE}

import (
    "testing"
)

type ${INTERFACE}TestSuite struct {
    Name          string
    Create${INTERFACE} func() ${INTERFACE}
    Cleanup       func()
}

func (suite *${INTERFACE}TestSuite) RunAllTests(t *testing.T) {
    if suite.Cleanup != nil {
        defer suite.Cleanup()
    }
    
    t.Run(suite.Name, func(t *testing.T) {
        suite.testBasicOperation(t)
        suite.testErrorHandling(t)
        suite.testConcurrentAccess(t)
    })
}

func (suite *${INTERFACE}TestSuite) testBasicOperation(t *testing.T) {
    t.Run("BasicOperation", func(t *testing.T) {
        instance := suite.Create${INTERFACE}()
        if instance == nil {
            t.Fatal("Failed to create ${INTERFACE} instance")
        }
        // Add specific tests here
    })
}

func (suite *${INTERFACE}TestSuite) testErrorHandling(t *testing.T) {
    t.Run("ErrorHandling", func(t *testing.T) {
        instance := suite.Create${INTERFACE}()
        // Test error scenarios
    })
}

func (suite *${INTERFACE}TestSuite) testConcurrentAccess(t *testing.T) {
    t.Run("ConcurrentAccess", func(t *testing.T) {
        instance := suite.Create${INTERFACE}()
        // Test concurrent access
    })
}
EOF

echo "Test suite generated: $SUITE_FILE"
echo "Next steps:"
echo "1. Implement specific test cases in each test function"
echo "2. Create implementation tests that use this suite"
echo "3. Run: go test -v ./${PACKAGE}"
```

## Test Naming Convention

```bash
Pattern: Test[Type]_[Method]_[Scenario]

Examples:
- TestFileStorage_WriteEvent_SucceedsWithValidEvent
- TestFileStorage_WriteEvent_FailsWhenDiskFull
- TestFileStorage_ReadAllEvents_RecoversFromCorruption
- TestMemoryCache_Get_ReturnsErrorWhenKeyNotFound
- TestEventCodec_Decode_DetectsCRC32Mismatch
```

## Coverage Requirements

```bash
#!/bin/bash
# .ai/scripts/check_coverage.sh

set -e

echo "Running tests with coverage..."

# Generate coverage profile
go test -coverprofile=coverage.out ./...

# Check overall coverage
OVERALL=$(go tool cover -func=coverage.out | grep total | awk '{print substr($3, 1, length($3)-1)}')

echo "Overall coverage: ${OVERALL}%"

if (( $(echo "$OVERALL < 80" | bc -l) )); then
    echo "Coverage below 80% threshold"
    exit 1
fi

# Check per-package coverage
echo "Per-package coverage:"
go tool cover -func=coverage.out | grep -v "total" | awk '{
    if ($3 != "") {
        coverage = substr($3, 1, length($3)-1)
        if (coverage < 80.0) {
            print $1 ": " coverage "% (below threshold)"
            exit 1
        } else {
            print $1 ": " coverage "%"
        }
    }
}'

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html

echo "Coverage report generated: coverage.html"
echo "All coverage requirements met!"
```

## Benchmark Template

```go
// Add benchmarks for performance-critical paths

func Benchmark[Type]_[Operation](b *testing.B) {
    instance := create[Type]()
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        instance.[Operation]()
    }
}

// Example:
func BenchmarkFileStorage_WriteEvent(b *testing.B) {
    storage := createFileStorage()
    event := createTestEvent()
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        storage.WriteEvent(event)
    }
}
```
