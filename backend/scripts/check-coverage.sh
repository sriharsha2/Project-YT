#!/usr/bin/env bash
# Runs the test suite and fails unless statement coverage is exactly 100%.
# cmd/ is excluded because it only wires dependencies together (skills.md §2.1).
set -euo pipefail

cd "$(dirname "$0")/.."

packages=$(go list ./... | grep -v '/cmd/')
go test -count=1 -coverprofile=coverage.out $packages

total=$(go tool cover -func=coverage.out | awk '/^total:/ {print $3}')
echo "total coverage: ${total}"

if [ "${total}" != "100.0%" ]; then
  echo "coverage gate failed: expected 100.0%, got ${total}" >&2
  go tool cover -func=coverage.out | grep -v '100.0%' >&2
  exit 1
fi
