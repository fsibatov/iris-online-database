#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
node --check web/app.js
python3 -m unittest discover -s tools -p 'test_*.py'

binary="${1:-dist/iris-online-linux-smoke}"
if [[ ! -x "$binary" ]]; then
  CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags '-buildid= -X main.appVersion=1.0.1 -X main.releaseMarker=IrisOnlineDiagnostic/1.0.1/go1.23.2' -o "$binary" .
fi
python3 tools/run_smoke_tests.py --binary "$binary"
