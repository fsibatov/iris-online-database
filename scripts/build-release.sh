#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
OUTPUT_DIR="${1:-}"
if [ -z "$OUTPUT_DIR" ]; then
  echo "Usage: scripts/build-release.sh /absolute/external/output-directory"
  exit 2
fi
mkdir -p "$OUTPUT_DIR"
OUTPUT_DIR="$(cd "$OUTPUT_DIR" && pwd -P)"
case "$OUTPUT_DIR/" in
  "$ROOT_DIR/"*)
    echo "Release output must be outside the source tree."
    exit 2
    ;;
esac

cd "$ROOT_DIR"
python3 -B tools/release_fingerprint.py --verify
VERSION="$(tr -d '[:space:]' < VERSION)"
HEAD="$(git rev-parse HEAD)"
EXPECTED_GO="$(tr -d '[:space:]' < .go-version)"
if [ "$(go env GOVERSION)" != "go$EXPECTED_GO" ]; then
  echo "Go $EXPECTED_GO is required."
  exit 1
fi
if [ "$(wails version | head -n 1 | tr -d '\r')" != "v2.14.0" ]; then
  echo "Wails CLI v2.14.0 is required."
  exit 1
fi

cleanup_generated() {
  rm -rf -- "$ROOT_DIR/build/bin" "$ROOT_DIR/build/generated" "$ROOT_DIR/web/wailsjs"
  rm -f -- "$ROOT_DIR/build/appicon.png"
}
trap cleanup_generated EXIT
cleanup_generated

BEFORE="$(git status --porcelain=v1 --untracked-files=all)"
wails build \
  -platform windows/amd64 \
  -webview2 embed \
  -trimpath \
  -clean \
  -skipbindings \
  -s \
  -nosyncgomod \
  -m \
  -o IrisOnlineDatabase.exe \
  -ldflags "-buildid= -X main.appVersion=$VERSION -X main.releaseMarker=IrisOnlineRelease/$VERSION/$HEAD"

ARTIFACT="$OUTPUT_DIR/iris-online-database-$VERSION-windows-amd64.exe"
cp "$ROOT_DIR/build/bin/IrisOnlineDatabase.exe" "$ARTIFACT"
(
  cd "$OUTPUT_DIR"
  sha256sum "$(basename "$ARTIFACT")" > SHA256SUMS.txt
)
cleanup_generated
AFTER="$(git status --porcelain=v1 --untracked-files=all)"
if [ "$BEFORE" != "$AFTER" ]; then
  echo "Build changed the source tree."
  exit 1
fi
python3 -B tools/verify_executables.py --directory "$OUTPUT_DIR" --version "$VERSION"
python3 -B tools/verify_windows_resources.py --directory "$OUTPUT_DIR" --version "$VERSION"
echo "Release build: PASS"
