#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
TASK_TMP="$(mktemp -d "${TMPDIR:-/tmp}/iris-release-gate.XXXXXXXX")"
AUDIT_ENV="${IRIS_AUDIT_ENV:-${TMPDIR:-/tmp}/iris-online-python-audit}"
PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:-${TMPDIR:-/tmp}/iris-online-playwright}"
export PLAYWRIGHT_BROWSERS_PATH
export PYTHONPYCACHEPREFIX="$TASK_TMP/pycache"

cleanup() {
  rm -rf -- "$TASK_TMP"
}
trap cleanup EXIT

cd "$ROOT_DIR"
. "$ROOT_DIR/scripts/release-tools.sh"
if ! INITIAL_STATUS="$(git status --porcelain=v1 --untracked-files=all)"; then
  echo "Git status failed before the RELEASE gate."
  exit 1
fi
if [ -n "$INITIAL_STATUS" ]; then
  echo "RELEASE gate requires a clean Git working tree."
  exit 1
fi
if [ "$(git branch --show-current)" != "main" ]; then
  echo "RELEASE gate requires branch main."
  exit 1
fi
BEFORE_HEAD="$(git rev-parse HEAD)"
BEFORE_STATUS="$(git status --porcelain=v1 --untracked-files=all)"
EXPECTED_GO="$(tr -d '[:space:]' < .go-version)"
ACTUAL_GO="$(go env GOVERSION)"
if [ "$ACTUAL_GO" != "go$EXPECTED_GO" ]; then
  echo "Go version mismatch: expected go$EXPECTED_GO, got $ACTUAL_GO"
  exit 1
fi
WAILS_BIN="$(iris_resolve_wails v2.14.0 || true)"
STATICCHECK_BIN="$(iris_resolve_staticcheck 2026.1 || true)"
GOVULNCHECK_BIN="$(iris_resolve_govulncheck v1.6.0 || true)"
GITLEAKS_BIN="$(iris_resolve_gitleaks 8.30.1 || true)"
if [ -z "$WAILS_BIN" ]; then
  echo "Pinned Wails CLI v2.14.0 could not be resolved from Go module metadata."
  exit 1
fi
if [ -z "$STATICCHECK_BIN" ]; then
  echo "Pinned Staticcheck 2026.1 could not be resolved."
  exit 1
fi
if [ -z "$GOVULNCHECK_BIN" ]; then
  echo "Pinned govulncheck v1.6.0 could not be resolved."
  exit 1
fi
if [ -z "$GITLEAKS_BIN" ]; then
  echo "Pinned Gitleaks 8.30.1 could not be resolved."
  exit 1
fi

python3 -B tools/repository_audit.py
UNFORMATTED="$(gofmt -l -- *.go)"
if [ -n "$UNFORMATTED" ]; then
  echo "$UNFORMATTED"
  exit 1
fi
go mod verify
go mod tidy -diff
go list -deps ./... > "$TASK_TMP/go-dependencies.txt"
timeout 10m go build -trimpath -o "$TASK_TMP/build-probe" .
timeout 15m go test -count=1 ./...
timeout 20m go test -race -count=1 ./...
timeout 10m go vet ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 timeout 15m "$STATICCHECK_BIN" ./...
timeout 15m "$GOVULNCHECK_BIN" ./...

REQ_HASH="$(python3 - <<'PY'
import hashlib
import platform
from pathlib import Path

requirements = Path("tools/requirements-audit.txt").read_bytes()
print(hashlib.sha256(requirements + platform.python_version().encode()).hexdigest())
PY
)"
MARKER="$AUDIT_ENV/.iris-requirements-sha256"
CURRENT_MARKER=""
if [ -f "$MARKER" ]; then
  CURRENT_MARKER="$(tr -d '[:space:]' < "$MARKER")"
fi
if [ "$CURRENT_MARKER" != "$REQ_HASH" ] || \
  ! "$AUDIT_ENV/bin/python" -B tools/verify_python_environment.py >/dev/null 2>&1 || \
  ! "$AUDIT_ENV/bin/python" -m pip check >/dev/null 2>&1; then
  python3 -m venv --clear "$AUDIT_ENV"
  for attempt in 1 2 3; do
    if timeout 8m "$AUDIT_ENV/bin/python" -m pip install \
      --disable-pip-version-check --requirement tools/requirements-audit.txt; then
      break
    fi
    if [ "$attempt" -eq 3 ]; then
      echo "Python audit environment installation failed."
      exit 1
    fi
  done
  printf '%s\n' "$REQ_HASH" > "$MARKER"
fi

"$AUDIT_ENV/bin/python" -B tools/verify_python_environment.py
"$AUDIT_ENV/bin/python" -m pip check
"$AUDIT_ENV/bin/python" -B tools/validate_workflows.py

"$AUDIT_ENV/bin/python" -B -m unittest discover -s tools -p 'test_*.py'
"$AUDIT_ENV/bin/ruff" check --no-cache .
"$AUDIT_ENV/bin/ruff" format --check --no-cache .
"$AUDIT_ENV/bin/bandit" -q -r tools -x 'tools/test_*.py'
timeout 10m "$AUDIT_ENV/bin/pip-audit" --local --cache-dir "$AUDIT_ENV/pip-audit-cache"
node --check web/app.js

python3 -B tools/data_presentation_audit.py
python3 -B tools/frontend_smoke_test.py

iris_test_gitleaks_detection "$GITLEAKS_BIN"
timeout 10m "$GITLEAKS_BIN" dir --no-banner --redact .
iris_gitleaks_history_scan "$GITLEAKS_BIN" .
python3 -B tools/repository_audit.py

AFTER_HEAD="$(git rev-parse HEAD)"
AFTER_STATUS="$(git status --porcelain=v1 --untracked-files=all)"
if [ "$BEFORE_HEAD" != "$AFTER_HEAD" ] || [ "$BEFORE_STATUS" != "$AFTER_STATUS" ]; then
  echo "RELEASE gate changed the source tree."
  exit 1
fi
python3 -B tools/release_fingerprint.py --write
echo "RELEASE gate: PASS"
