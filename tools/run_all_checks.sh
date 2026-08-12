#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

VERSION="1.1.0"
ACTUAL_GO="$(go version | awk '{print $3}')"
temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/iris-online-checks.XXXXXX")"
cleanup() { rm -rf "$temp_dir"; }
trap cleanup EXIT

python3 -B tools/repository_audit.py

files="$(gofmt -l -- *.go)"
if [[ -n "$files" ]]; then
  printf '%s\n' "$files" >&2
  exit 1
fi
go mod verify
if ! go mod tidy -diff >/dev/null; then
  echo "go mod tidy -diff обнаружил изменения go.mod/go.sum." >&2
  go mod tidy -diff >&2 || true
  exit 1
fi
go build -o "$temp_dir/build-probe" .
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
node --check web/app.js
python3 -B -m unittest discover -s tools -p 'test_*.py'

python3 - <<'PY_CHECK_DEPS'
from importlib.metadata import PackageNotFoundError, version
from pathlib import Path

requirements = Path("tools/requirements-audit.txt")
problems = []
for raw in requirements.read_text(encoding="utf-8").splitlines():
    line = raw.strip()
    if not line or line.startswith("#"):
        continue
    if "==" not in line:
        problems.append(f"неподдерживаемая запись зависимости: {line}")
        continue
    package, expected = (part.strip() for part in line.split("==", 1))
    try:
        actual = version(package)
    except PackageNotFoundError:
        problems.append(f"{package}: не установлена (требуется {expected})")
        continue
    if actual != expected:
        problems.append(f"{package}: установлена {actual}, требуется {expected}")
if problems:
    raise SystemExit(
        "Несовместимое окружение smoke-тестов: "
        + "; ".join(problems)
        + ". Выполните: python3 -m pip install -r tools/requirements-audit.txt"
    )
PY_CHECK_DEPS

binary="${1:-$temp_dir/iris-online-smoke}"

if [[ ! -x "$binary" ]]; then
  mkdir -p "$(dirname "$binary")"
  CGO_ENABLED=0 go build -buildvcs=false -trimpath \
    -ldflags "-buildid= -X main.appVersion=$VERSION -X main.releaseMarker=IrisOnlineDiagnostic/$VERSION/$ACTUAL_GO" \
    -o "$binary" .
fi
python3 -B tools/run_smoke_tests.py --binary "$binary"
