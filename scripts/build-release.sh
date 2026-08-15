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
. "$ROOT_DIR/scripts/release-tools.sh"
python3 -B tools/release_fingerprint.py --verify
VERSION="$(tr -d '[:space:]' < VERSION)"
HEAD="$(git rev-parse HEAD)"
EXPECTED_GO="$(tr -d '[:space:]' < .go-version)"
if [ "$(go env GOVERSION)" != "go$EXPECTED_GO" ]; then
  echo "Go $EXPECTED_GO is required."
  exit 1
fi
WAILS_BIN="$(iris_resolve_wails v2.14.0 || true)"
if [ -z "$WAILS_BIN" ]; then
  echo "Pinned Wails CLI v2.14.0 could not be resolved from Go module metadata."
  exit 1
fi

cleanup_generated() {
  rm -rf -- "$ROOT_DIR/build/bin" "$ROOT_DIR/build/generated" "$ROOT_DIR/web/wailsjs"
  rm -f -- "$ROOT_DIR/build/appicon.png"
}
trap cleanup_generated EXIT
cleanup_generated

BEFORE="$(git status --porcelain=v1 --untracked-files=all)"

build_target() {
  platform="$1"
  suffix="$2"
  level_name="$3"
  level_value="$4"
  artifact="$OUTPUT_DIR/IrisOnlineDB-$VERSION-Windows-$suffix.exe"

  env -u GOAMD64 -u GO386 -u GOARM64 \
    CGO_ENABLED=0 "$level_name=$level_value" \
    "$WAILS_BIN" build \
      -platform "$platform" \
      -webview2 embed \
      -trimpath \
      -clean \
      -skipbindings \
      -s \
      -nosyncgomod \
      -m \
      -o IrisOnlineDatabase.exe \
      -ldflags "-buildid= -X main.appVersion=$VERSION -X main.releaseMarker=IrisOnlineRelease/$VERSION/$HEAD"

  cp "$ROOT_DIR/build/bin/IrisOnlineDatabase.exe" "$artifact"
}

rm -f -- \
  "$OUTPUT_DIR/IrisOnlineDB-$VERSION-Windows-x64.exe" \
  "$OUTPUT_DIR/IrisOnlineDB-$VERSION-Windows-x86.exe" \
  "$OUTPUT_DIR/IrisOnlineDB-$VERSION-Windows-arm64.exe" \
  "$OUTPUT_DIR/SHA256SUMS.txt"

build_target windows/amd64 x64 GOAMD64 v1
build_target windows/386 x86 GO386 sse2
build_target windows/arm64 arm64 GOARM64 v8.0

(
  cd "$OUTPUT_DIR"
  sha256sum \
    "IrisOnlineDB-$VERSION-Windows-x64.exe" \
    "IrisOnlineDB-$VERSION-Windows-x86.exe" \
    "IrisOnlineDB-$VERSION-Windows-arm64.exe" > SHA256SUMS.txt
)
cleanup_generated
AFTER="$(git status --porcelain=v1 --untracked-files=all)"
if [ "$BEFORE" != "$AFTER" ]; then
  echo "Build changed the source tree."
  exit 1
fi
python3 -B tools/verify_release_assets.py --directory "$OUTPUT_DIR" --version "$VERSION"
python3 -B tools/verify_executables.py --directory "$OUTPUT_DIR" --version "$VERSION"
python3 -B tools/verify_windows_resources.py --directory "$OUTPUT_DIR" --version "$VERSION"
[ "$(git rev-parse HEAD)" = "$HEAD" ] || { echo "HEAD changed during release artifact build."; exit 1; }
python3 -B tools/release_fingerprint.py --verify
echo "Release build: PASS (Windows x64, x86, arm64)"
