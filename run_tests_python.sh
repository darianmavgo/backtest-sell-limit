#!/bin/bash

# Run tests for the backtest-sell-limit project

echo "🧪 Running Go tests..."

# Run all tests with verbose output
go test -v ./cmd/web/...

# Check if tests passed
if [ $? -eq 0 ]; then
    echo "✅ All tests passed!"
else
    echo "❌ Some tests failed!"
    exit 1
fi

echo ""
echo "🧪 Running Python tests (if available)..."

# Check if Python tests exist and run them
if [ -f "test_*.py" ] || [ -d "tests/" ]; then
    if command -v pytest &> /dev/null; then
        pytest -v
    elif command -v python3 -m pytest &> /dev/null; then
        python3 -m pytest -v
    else
        echo "⚠️  pytest not found, skipping Python tests"
    fi
else
    echo "ℹ️  No Python tests found"
fi

echo ""
echo "🏁 Test run complete!"