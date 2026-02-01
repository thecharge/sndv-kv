
# Code Generator Agent

## Responsibility

Generate production Go code with zero ambiguity in naming.

## Input

Feature request, interface specification, or component description

## Output

- Interface definition (contract)
- Implementation (concrete struct)
- Test suite (interface tests)
- Configuration (with units)

## Execution Checklist

Before generating any code:

1. Identify all numeric types and their units
2. List all operations and their complete actions
3. Define error scenarios and their contexts
4. Plan test coverage strategy

During code generation:

1. Start with interface (contract-first design)
2. Add unit suffixes to all variables (InBytes, InMilliseconds, AsPercentage)
3. Use complete action verbs in functions (ProcessAndUpdate, WriteWithIntegrity)
4. Include full error context (what, where, why, original error)
5. Apply early return pattern (validate first, execute last)
6. Generate configuration with human-readable units

After code generation:

1. Run automated quality checks
2. Verify test coverage threshold
3. Check for ambiguous naming
4. Validate error context completeness

## Quality Checks

Run these commands automatically:

```bash
#!/bin/bash
# .ai/scripts/quality_check.sh

set -e

echo "Running code quality checks..."

# Format check
echo "Checking formatting..."
if ! gofmt -l . | grep -q "^$"; then
    echo "Format violations found. Run: gofmt -w ."
    exit 1
fi

# Spell check (code and comments)
echo "Checking spelling..."
if ! command -v codespell &> /dev/null; then
    echo "Installing codespell..."
    pip install codespell
fi
codespell --skip=".git,vendor,*.sum" \
    --ignore-words=.ai/config/codespell_ignore.txt \
    --check-filenames \
    --check-hidden

# Vet check
echo "Running go vet..."
go vet ./...

# Lint check
echo "Running linter..."
if ! command -v golangci-lint &> /dev/null; then
    echo "Installing golangci-lint..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi
golangci-lint run --config .ai/config/golangci.yml

# Race detection
echo "Running race detector..."
go test -race -short ./...

# Coverage check
echo "Checking test coverage..."
go test -cover ./... | awk '
    /coverage:/ {
        coverage = substr($5, 1, length($5)-1)
        if (coverage < 80.0) {
            print "Coverage too low:", $2, coverage "%"
            exit 1
        }
    }
'

# Ambiguous naming check
echo "Checking for ambiguous names..."
if grep -rn "var size " --include="*.go" .; then
    echo "Found ambiguous 'size' variable. Add unit suffix (sizeInBytes, etc.)"
    exit 1
fi

if grep -rn "var max " --include="*.go" .; then
    echo "Found ambiguous 'max' variable. Add unit suffix (maximumSizeInBytes, etc.)"
    exit 1
fi

if grep -rn "var timeout " --include="*.go" .; then
    echo "Found ambiguous 'timeout' variable. Add unit suffix (timeoutInMilliseconds, etc.)"
    exit 1
fi

# Context-free error check
echo "Checking for errors without context..."
if grep -rn "return err$" --include="*.go" .; then
    echo "Found errors without context. Wrap with fmt.Errorf(..., err)"
    exit 1
fi

echo "All quality checks passed!"
```

## Example Generation

Input:

```text
Create event storage for distributed TODO system
```

Output files:

`event_storage.go`:

```go
package storage

type EventStorage interface {
    WriteEventToSegmentFile(event *Event) error
    ReadAllEventsWithCallback(processEvent func(*Event) error) error
    GetCurrentStatistics() StorageStatistics
    CloseAndFlushBuffers() error
}

type StorageStatistics struct {
    TotalEventsWrittenCount   int64
    TotalEventsReadCount      int64
    CorruptedEventsFoundCount int64
    TotalBytesWritten         int64
}
```

`file_storage.go`:

```go
package storage

type FileBasedEventStorage struct {
    storageDirectoryPath              string
    maximumSegmentSizeInBytes         int64
    currentSegmentSizeInBytes         int64
    maximumCorruptionRateAsPercentage float64
    // ... implementation
}
```

`storage_config.go`:

```go
package storage

type StorageConfiguration struct {
    StorageDirectoryPath              string  `json:"storage_directory_path"`
    MaximumSegmentSizeInMegabytes     int     `json:"maximum_segment_size_megabytes"`
    WriteBufferSizeInKilobytes        int     `json:"write_buffer_size_kilobytes"`
    MaximumCorruptionRateAsPercentage float64 `json:"maximum_corruption_rate_percent"`
}
```

`storage_test.go`:

```go
package storage

type EventStorageTestSuite struct {
    CreateStorage func() EventStorage
}

func (suite *EventStorageTestSuite) RunAllTests(t *testing.T) {
    suite.testWriteAndReadSingleEvent(t)
    suite.testWriteAndReadMultipleEvents(t)
    suite.testCorruptionDetectionWithCRC32Mismatch(t)
    suite.testRecoveryAfterCorruptedFrame(t)
    suite.testConcurrentWritesFromMultipleGoroutines(t)
}
```

## Naming Rules Reference

Variables must include:

- Units for numeric types: InBytes, InMilliseconds, AsPercentage, InNanoseconds
- Scope: current, maximum, minimum, total, average
- Type clarification: Count, Rate, Size, Duration

Functions must include:

- Complete action: Process, Write, Read, Update, Delete (never alone)
- Object being acted upon: Event, File, Buffer, Cache
- Qualifying details: WithIntegrity, AndUpdateCache, FromSegmentFile

Configuration must use:

- Megabytes not bytes (100 not 104857600)
- Milliseconds not nanoseconds (5000 not 5000000000)
- Percentage not ratio (1.0 not 0.01)
