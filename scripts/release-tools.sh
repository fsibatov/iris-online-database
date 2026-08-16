# Shared fail-closed resolver for pinned Go-installed release tools.
# This file is sourced by release-gate.sh and build-release.sh.

export GOTOOLCHAIN=local

iris_go_bin_dir() {
  local gobin gopath
  gobin="$(go env GOBIN)"
  if [ -n "$gobin" ]; then
    printf '%s\n' "$gobin"
    return 0
  fi
  gopath="$(go env GOPATH)"
  gopath="${gopath%%:*}"
  [ -n "$gopath" ] || return 1
  printf '%s/bin\n' "$gopath"
}

iris_tool_candidates() {
  local name="$1" gobin path_candidate
  gobin="$(iris_go_bin_dir 2>/dev/null || true)"
  if [ -n "$gobin" ]; then
    printf '%s\n' "$gobin/$name"
  fi
  path_candidate="$(command -v "$name" 2>/dev/null || true)"
  if [ -n "$path_candidate" ] && [ "$path_candidate" != "$gobin/$name" ]; then
    printf '%s\n' "$path_candidate"
  fi
}

iris_resolve_wails() {
  local expected="$1" candidate metadata
  while IFS= read -r candidate; do
    [ -n "$candidate" ] || continue
    [ -x "$candidate" ] || continue
    metadata="$(go version -m "$candidate" 2>/dev/null || true)"
    printf '%s\n' "$metadata" | grep -Fxq $'path\tgithub.com/wailsapp/wails/v2/cmd/wails' || continue
    printf '%s\n' "$metadata" | awk -v expected="$expected" '$1 == "mod" && $2 == "github.com/wailsapp/wails/v2" && $3 == expected { found = 1 } END { exit !found }' || continue
    printf '%s\n' "$candidate"
    return 0
  done < <(iris_tool_candidates wails)
  return 1
}

iris_resolve_staticcheck() {
  local expected="$1" candidate line version
  while IFS= read -r candidate; do
    [ -n "$candidate" ] || continue
    [ -x "$candidate" ] || continue
    line="$("$candidate" -version 2>&1 | head -n 1 | tr -d '\r')" || continue
    version="$(printf '%s\n' "$line" | awk '{print $2}')"
    [ "$version" = "$expected" ] || continue
    printf '%s\n' "$candidate"
    return 0
  done < <(iris_tool_candidates staticcheck)
  return 1
}

iris_resolve_govulncheck() {
  local expected="$1" candidate output version
  while IFS= read -r candidate; do
    [ -n "$candidate" ] || continue
    [ -x "$candidate" ] || continue
    output="$("$candidate" -version 2>&1)" || continue
    version="$(printf '%s\n' "$output" | sed -n 's/.*Scanner:[[:space:]]*govulncheck@\(v[^[:space:]]*\).*/\1/p' | head -n 1)"
    [ "$version" = "$expected" ] || continue
    printf '%s\n' "$candidate"
    return 0
  done < <(iris_tool_candidates govulncheck)
  return 1
}

iris_resolve_gitleaks() {
  local expected="$1" candidate version
  candidate="$(command -v gitleaks 2>/dev/null || true)"
  [ -n "$candidate" ] || return 1
  [ -x "$candidate" ] || return 1
  version="$("$candidate" version 2>/dev/null | head -n 1 | tr -d '\r')" || return 1
  [ "$version" = "$expected" ] || return 1
  printf '%s\n' "$candidate"
}

iris_test_gitleaks_detection() {
  local executable="$1" probe_dir exit_code
  probe_dir="$(mktemp -d "${TMPDIR:-/tmp}/iris-gitleaks-probe.XXXXXXXX")"
  printf 'token=%s%s\n' 'ghp_' 'wA9mK2pLxN4vRtQzY6bC8dEfGhJlM0oPq1rS' > "$probe_dir/probe.txt"
  set +e
  "$executable" dir --no-banner --redact --exit-code 37 "$probe_dir" >/dev/null 2>&1
  exit_code=$?
  set -e
  rm -rf -- "$probe_dir"
  if [ "$exit_code" -ne 37 ]; then
    echo "Gitleaks functional detection self-test failed." >&2
    return 1
  fi
}

iris_gitleaks_history_scan() {
  local executable="$1" repository="$2" output exit_code
  output="$(mktemp "${TMPDIR:-/tmp}/iris-gitleaks-history.XXXXXXXX")"
  set +e
  timeout 10m "$executable" git --no-banner --redact --log-level info "$repository" >"$output" 2>&1
  exit_code=$?
  set -e
  cat "$output"
  if [ "$exit_code" -ne 0 ]; then
    rm -f -- "$output"
    echo "Gitleaks Git-history scan failed." >&2
    return "$exit_code"
  fi
  if ! grep -Eq '[1-9][0-9]* commits scanned\.' "$output"; then
    rm -f -- "$output"
    echo "Gitleaks Git-history scan did not prove that any commits were scanned." >&2
    return 1
  fi
  rm -f -- "$output"
}
