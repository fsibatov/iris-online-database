#!/usr/bin/env bash
set -euo pipefail

VERSION="1.0.1"
EXPECTED_GO="go$(tr -d '[:space:]' < .go-version)"
ACTUAL_GO="$(go version | awk '{print $3}')"
DIAGNOSTIC=0
if [[ "$ACTUAL_GO" != "$EXPECTED_GO" ]]; then
  DIAGNOSTIC=1
  if [[ "${IRIS_ALLOW_UNSUPPORTED_GO:-}" != "1" ]]; then
    echo "Требуется $EXPECTED_GO, обнаружен $ACTUAL_GO. Для диагностической сборки задайте IRIS_ALLOW_UNSUPPORTED_GO=1." >&2
    exit 1
  fi
fi

if [[ "$DIAGNOSTIC" == "0" && "${IRIS_SKIP_CHECKS:-}" == "1" ]]; then
  echo "IRIS_SKIP_CHECKS=1 разрешён только для диагностической сборки. Публикационная сборка должна пройти все проверки." >&2
  exit 1
fi

if [[ "${IRIS_SKIP_CHECKS:-}" != "1" ]]; then
  go test -count=1 ./...
  go test -race -count=1 ./...
  go vet ./...
  node --check web/app.js
  python3 -m unittest discover -s tools -p 'test_*.py'
fi

mkdir -p dist
if [[ "$DIAGNOSTIC" == "1" ]]; then
  MARKER="IrisOnlineDiagnostic/$VERSION/$ACTUAL_GO"
  NAME_SUFFIX="-diagnostic-$ACTUAL_GO"
else
  MARKER="IrisOnlineRelease/$VERSION"
  NAME_SUFFIX=""
fi
LDFLAGS="-s -w -H windowsgui -buildid= -X main.appVersion=$VERSION -X main.releaseMarker=$MARKER"

build_target() {
  local arch="$1" name="$2"
  shift 2
  env CGO_ENABLED=0 GOOS=windows GOARCH="$arch" "$@" \
    go build -buildvcs=false -trimpath -ldflags "$LDFLAGS" \
    -o "dist/IrisOnlineDB-$VERSION$NAME_SUFFIX-Windows-$name.exe" .
}

build_target amd64 x64 GOAMD64=v1
build_target 386 x86 GO386=softfloat
build_target arm64 arm64

if [[ "$DIAGNOSTIC" == "1" ]]; then
  echo "Собрано Iris Online ${VERSION}: диагностические Windows x64, x86 и ARM64 ($ACTUAL_GO)."
else
  echo "Собрано Iris Online ${VERSION}: Windows x64, x86 и ARM64 ($ACTUAL_GO)."
fi
