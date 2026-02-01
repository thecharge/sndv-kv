#!/bin/bash
# Install all required development tools

set -e

echo "Installing development tools..."

# Go tools
echo "Installing Go linters..."
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

echo "Installing gocyclo..."
go install github.com/fzipp/gocyclo/cmd/gocyclo@latest

# Python tools for scripts
echo "Installing Python spell checker..."
if ! command -v pip3 &> /dev/null; then
    echo "pip3 not found. Please install Python 3 first."
    exit 1
fi

pip3 install codespell

# Git hooks
echo "Setting up Git hooks..."
mkdir -p .git/hooks

cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash

set -e

echo "Running pre-commit checks..."

# Run quality checks
./.ai/scripts/quality_check.sh

# Run tests
go test -short ./...

echo "Pre-commit checks passed!"
EOF

chmod +x .git/hooks/pre-commit

echo "All tools installed successfully!"
echo ""
echo "Installed:"
echo "  - golangci-lint"
echo "  - gocyclo"
echo "  - codespell"
echo "  - Git pre-commit hook"
echo ""
echo "Next steps:"
echo "  1. Run: .ai/scripts/quality_check.sh"
echo "  2. Commit will automatically run checks"