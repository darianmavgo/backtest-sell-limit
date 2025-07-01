#!/bin/bash

# Development helper script for backtest-sell-limit

show_help() {
    echo "📚 Backtest Sell Limits Development Helper"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  start       Start the server with hot reloading (default)"
    echo "  dev         Start with hot reloading AND run tests on file changes"
    echo "  test        Run all tests once"
    echo "  build       Build the application"
    echo "  clean       Clean temporary files"
    echo "  help        Show this help message"
    echo ""
    echo "Examples:"
    echo "  ./dev.sh          # Start with hot reloading"
    echo "  ./dev.sh dev      # Start with hot reloading + tests"
    echo "  ./dev.sh test     # Run tests once"
}

case "${1:-start}" in
    "start")
        echo "🚀 Starting development server with hot reloading..."
        air
        ;;
    "dev")
        echo "🔥 Starting development server with hot reloading + tests..."
        air -c .air.dev.toml
        ;;
    "test")
        echo "🧪 Running tests..."
        ./run_tests.sh
        ;;
    "build")
        echo "🔨 Building application..."
        go build -o tmp/main cmd/web/main.go cmd/web/config.go cmd/web/routes.go
        echo "✅ Build complete: tmp/main"
        ;;
    "clean")
        echo "🧹 Cleaning temporary files..."
        rm -rf tmp/
        mkdir -p tmp/
        echo "✅ Clean complete"
        ;;
    "help"|"-h"|"--help")
        show_help
        ;;
    *)
        echo "❌ Unknown command: $1"
        echo ""
        show_help
        exit 1
        ;;
esac