#!/bin/bash
GOARCH=arm64 go build -o ./bin/servebacktest ./cmd/web
codesign -f -s - ./bin/servebacktest
echo "Build complete: bin/servebacktest"
