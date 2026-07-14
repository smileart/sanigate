#!/usr/bin/env bash
#
# run-scripts.sh — build sanigate and assert its verdicts against known inputs.
#
# Two evals, both piping into `sanigate -p` (porcelain: analysis only, nothing is
# ever executed). The assertion reads sanigate's exit code, keyed to INTENT:
#
#   0 = benign (safe, or legit-but-powerful)   3 = suspicious   4 = malicious
#   1/2 = tool error
#
# The point of the two-axis model is that capability never raises the exit code, so a
# legitimate installer with a high blast radius still exits 0. This eval checks the
# separation actually holds:
#
#   fixtures   ./scripts/*.sh — good.sh is benign; the rest are malware and must flag.
#   installers real-world `curl | sh` installers — must come back benign (exit 0),
#              NOT flagged, even though they download and run binaries. Opt-in
#              (network) via `--installers`, since it fetches remote URLs.
#
# The key comes from SNGT_OPENAI_API_KEY or the sanigate config file.
#
# Usage:
#   ./run-scripts.sh                 # fixtures: build + assert every ./scripts/*.sh
#   ./run-scripts.sh evil.sh good.sh # fixtures: a subset
#   ./run-scripts.sh --installers    # network: assert real installers read as benign
#   SNGT_MODEL=gpt-4o ./run-scripts.sh
#
set -euo pipefail

cd "$(dirname "$0")"

bin="bin/sanigate"
scripts_dir="scripts"

if [[ -z "${SNGT_OPENAI_API_KEY:-}" ]]; then
  echo "note: SNGT_OPENAI_API_KEY not set — relying on the sanigate config file for the key" >&2
fi

# label_for <exit-code> -> human-readable verdict
label_for() {
  case "$1" in
    0) echo benign ;;
    3) echo suspicious ;;
    4) echo malicious ;;
    *) echo "error($1)" ;;
  esac
}

pass=0
fail=0
failed_names=()

record() { # <ok bool> <name> <detail>
  if [[ "$1" == true ]]; then
    echo ">> PASS: $2 — $3"
    pass=$((pass + 1))
  else
    echo ">> FAIL: $2 — $3"
    fail=$((fail + 1))
    failed_names+=("$2")
  fi
}

echo "==> building $bin"
go build -o "$bin" .

# ─── Real-world installer eval (network, opt-in) ─────────────────────────────────
if [[ "${1:-}" == "--installers" ]]; then
  # label|url — all expected to read as benign (exit 0); many are high-capability.
  installers=(
    "webi|https://webi.sh/webi"
    "starship|https://starship.rs/install.sh"
    "golangci-lint|https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh"
    "benthos|https://sh.benthos.dev"
    "devbox|https://get.jetify.com/devbox"
    "tea|https://tea.xyz"
    "rustup|https://sh.rustup.rs"
    "casaos|https://get.casaos.io"
  )
  for entry in "${installers[@]}"; do
    name="${entry%%|*}"; url="${entry#*|}"
    echo
    echo "════════════════════════════════════════════════════════════════"
    echo "  installer: $name  (expected: benign / exit 0)"
    echo "  $url"
    echo "════════════════════════════════════════════════════════════════"
    if ! body=$(curl -fsSL --max-time 30 "$url" 2>/dev/null); then
      record false "$name" "FETCH FAILED (dead/moved URL or network)"
      continue
    fi
    code=0
    printf '%s' "$body" | "$bin" -p || code=$?
    actual="$(label_for "$code")"
    # An installer must not be flagged: exit 0 (benign) passes.
    [[ "$code" -eq 0 ]] && ok=true || ok=false
    record "$ok" "$name" "verdict '$actual' (exit $code)"
  done

# ─── Local fixture eval (offline) ────────────────────────────────────────────────
else
  targets=()
  if [[ $# -gt 0 ]]; then
    for arg in "$@"; do
      if [[ -f "$arg" ]]; then targets+=("$arg"); else targets+=("$scripts_dir/$arg"); fi
    done
  else
    for f in "$scripts_dir"/*.sh; do targets+=("$f"); done
  fi

  for f in "${targets[@]}"; do
    if [[ ! -f "$f" ]]; then
      echo "skip: $f (not found)" >&2
      continue
    fi
    base="$(basename "$f")"
    # good.sh is the only benign fixture; every other one is malware by design.
    if [[ "$base" == "good.sh" ]]; then expected=benign; else expected=flagged; fi

    echo
    echo "════════════════════════════════════════════════════════════════"
    echo "  $f  (expected: $expected)"
    echo "════════════════════════════════════════════════════════════════"

    code=0
    "$bin" -p < "$f" || code=$?
    actual="$(label_for "$code")"

    # benign fixtures must exit 0; flagged fixtures must exit 3 or 4.
    ok=false
    case "$expected" in
      benign)  [[ "$code" -eq 0 ]] && ok=true ;;
      flagged) [[ "$code" -eq 3 || "$code" -eq 4 ]] && ok=true ;;
    esac
    record "$ok" "$base" "expected $expected, verdict '$actual' (exit $code)"
  done
fi

echo
echo "════════════════════════════════════════════════════════════════"
echo "  RESULT: $pass passed, $fail failed"
if [[ $fail -gt 0 ]]; then
  echo "  failed: ${failed_names[*]}"
fi
echo "════════════════════════════════════════════════════════════════"

[[ $fail -eq 0 ]]
