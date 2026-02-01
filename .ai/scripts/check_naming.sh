#!/bin/bash
# Check for ambiguous naming violations

set -e

VIOLATIONS=0

echo "Checking for naming violations..."

# Check for ambiguous variable names
echo "Checking variable names..."

AMBIGUOUS_VARS="size max min timeout buffer data temp count total"

for var in $AMBIGUOUS_VARS; do
    matches=$(grep -rn "var $var " --include="*.go" . 2>/dev/null || true)
    if [ -n "$matches" ]; then
        echo "VIOLATION: Ambiguous variable '$var' found:"
        echo "$matches"
        echo "Add unit suffix (e.g., ${var}InBytes, ${var}InMilliseconds)"
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
done

# Check for ambiguous function names
echo "Checking function names..."

AMBIGUOUS_FUNCS="Process Handle Do Run Execute Get Set Update"

for func in $AMBIGUOUS_FUNCS; do
    matches=$(grep -rn "^func $func(" --include="*.go" . 2>/dev/null || true)
    if [ -n "$matches" ]; then
        echo "VIOLATION: Vague function name '$func' found:"
        echo "$matches"
        echo "Use complete action description (e.g., ProcessEventAndUpdateCache)"
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
done

# Check for context-free errors
echo "Checking error handling..."

matches=$(grep -rn "return err$" --include="*.go" . 2>/dev/null || true)
if [ -n "$matches" ]; then
    echo "VIOLATION: Errors without context found:"
    echo "$matches"
    echo "Wrap errors with fmt.Errorf to add context"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check for emoji in code
echo "Checking for emoji in code..."

matches=$(grep -rn "[😀-🙏🌀-🗿🚀-🛿]" --include="*.go" . 2>/dev/null || true)
if [ -n "$matches" ]; then
    echo "VIOLATION: Emoji found in code:"
    echo "$matches"
    echo "Remove all emoji from code and comments"
    VIOLATIONS=$((VIOLATIONS + 1))
fi

# Check for marker comments
echo "Checking for marker comments..."

MARKERS="TODO FIXME HACK CRITICAL XXX NOTE WARNING NOTE FIX"

for marker in $MARKERS; do
    matches=$(grep -rn "// $marker" --include="*.go" . 2>/dev/null || true)
    if [ -n "$matches" ]; then
        echo "VIOLATION: Marker comment '$marker' found:"
        echo "$matches"
        echo "Remove marker comments or replace with descriptive comments"
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
done

if [ $VIOLATIONS -gt 0 ]; then
    echo "Total naming violations: $VIOLATIONS"
    exit 1
else
    echo "No naming violations found!"
    exit 0
fi