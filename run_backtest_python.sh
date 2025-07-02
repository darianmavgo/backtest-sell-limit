#!/bin/bash

# run_backtest.sh - Wrapper script to run portfolio backtest with automatic lock cleanup
# Usage: ./run_backtest.sh

echo "🚀 Starting Portfolio Backtest"
echo "=============================="

# Clear any existing database locks before starting
echo "🧹 Clearing any existing database locks..."
if [ -f "./kill_db_locks.sh" ]; then
    ./kill_db_locks.sh
else
    echo "⚠️  kill_db_locks.sh not found, but Python will handle locks automatically"
fi

echo ""
echo "📊 Running portfolio backtest..."
echo "Press Ctrl+C to stop the backtest"
echo ""

# Clear Python cache to ensure latest code
rm -rf __pycache__ 2>/dev/null

# Run the backtest
python3 portfolio_backtest.py

echo ""
echo "🎉 Backtest completed!"
echo "" 