#!/usr/bin/env bash
#
# Regenerate the README demo GIFs from live sanigate runs.
#
# Requires: asciinema + agg (`brew install asciinema agg`) and a built ./bin/sanigate.
# The API key is read from the sanigate config file (SNGT_OPENAI_API_KEY is unset
# per run so the config key is used). Run from anywhere; widen your terminal to
# ~140 columns for the cleanest output.
#
set -euo pipefail
cd "$(dirname "$0")/.."

go build -o bin/sanigate .

# gruvbox-dark palette (agg custom format: background,foreground,color0..7)
THEME="282828,ebdbb2,3c3836,fb4934,b8bb26,fabd2f,83a598,d3869b,8ec07c,ebdbb2"

demo() { # <name> <shown-cmd> <real-cmd>
  local name="$1" shown="$2" real="$3" cast
  cast="$(mktemp -t "sanigate-$name").cast"
  env -u SNGT_OPENAI_API_KEY asciinema rec --overwrite \
    -c "unset SNGT_OPENAI_API_KEY; printf '\$ %s\n\n' '$shown'; $real" "$cast"
  agg --theme "$THEME" --font-size 26 --idle-time-limit 2 "$cast" "img/$name.gif"
  rm -f "$cast"
  echo "wrote img/$name.gif"
}

demo bad      'cat scripts/bad.sh | sanigate -p'              'cat scripts/bad.sh | ./bin/sanigate -p'
demo doot-kit 'cat scripts/doot-kit.sh | sanigate -p'         'cat scripts/doot-kit.sh | ./bin/sanigate -p'
demo webi     'curl -fsSL https://webi.sh/webi | sanigate -p' 'curl -fsSL https://webi.sh/webi | ./bin/sanigate -p'
