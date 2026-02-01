#!/bin/bash
# Enforce all code standards before allowing commit

set -e

echo "Enforcing code standards..."

# Spell check
echo "Step 1: Spell checking..."
codespell --skip=".git,vendor,*.sum" \
    --ignore-words=.ai/config/codespell_ignore.txt \
    --check-filenames \
    --check-hidden || {
    echo "Spell check failed. Fix errors and try again."
    exit 1
}

# Format check
echo "Step 2: Format checking..."
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
    echo "The following files are not formatted:"
    echo "$unformatted"
    echo "Run: gofmt -w ."
    exit 1
fi

# Naming validation
echo "Step 3: Checking naming standards..."
bash .ai/scripts/check_naming.sh || {
    echo "Naming standard violations found"
    exit 1
}

# Linting
echo "Step 4: Running linters..."
golangci-lint run --config .ai/config/golangci.yml || {
    echo "Linting failed"
    exit 1
}

# Tests
echo "Step 5: Running tests..."
go test -race -short ./... || {
    echo "Tests failed"
    exit 1
}

# Coverage
echo "Step 6: Checking coverage..."
bash .ai/scripts/check_coverage.sh || {
    echo "Coverage requirements not met"
    exit 1
}

# Violation scoring
echo "Step 7: Calculating severity score..."
bash .ai/scripts/detect_violations.sh || {
    echo "Code quality violations exceed threshold"
    exit 1
}

echo "All standards enforced! Code ready to commit."