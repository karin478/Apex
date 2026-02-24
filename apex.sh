#!/bin/bash
cd "$(dirname "$0")"
echo "Building apex..."
go build -o /tmp/apex_dev ./cmd/apex/ 2>&1 | grep -v "warning:"
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi
echo "Build OK. Launching..."
echo ""
exec /tmp/apex_dev "$@"
