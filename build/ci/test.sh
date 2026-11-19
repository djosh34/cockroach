#!/usr/bin/env bash
# Runs one shard of the tests.
#
# Usage: build/ci/test.sh test|logic <shard> <shards> [go test flags...]
#
# "test" runs the Go test packages, "logic" runs the logic tests (one Go test
# per logic test file). The items are spread over the shards by the times
# measured on CI, listed in build/ci/times.txt (seconds per item; an item not
# listed counts as 60 seconds): the longest item first, each to the shard with
# the least work so far.
set -euo pipefail
cd "$(dirname "$0")/../.."

kind=$1 shard=$2 shards=$3
shift 3

LOGIC_PKGS=(
  pkg/sql/logictest/tests/local
  pkg/sql/logictest/tests/fakedist
  pkg/sql/opt/exec/execbuilder/tests/local
)

items() {
  case $kind in
    test)
      go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.Dir}}{{end}}' ./pkg/... |
        sed "s|^$PWD/||" |
        grep -v -e '^pkg/sql/logictest/tests/' -e '^pkg/sql/opt/exec/execbuilder/tests/' \
          -e '^pkg/testutils/lint$'
      ;;
    logic)
      local p
      for p in "${LOGIC_PKGS[@]}"; do
        grep -o '^func Test[A-Za-z0-9_]*' "$p/generated_test.go" | sed "s|^func |$p:|"
      done
      ;;
    *)
      echo "unknown kind $kind" >&2
      exit 1
      ;;
  esac
}

# pick prints the items of this shard.
pick() {
  items |
    awk 'FNR == NR { if ($1 !~ /^#/) t[$1] = $2; next }
         { print (($1 in t) ? t[$1] : 60), $1 }' build/ci/times.txt - |
    sort -k1,1nr -k2,2 |
    awk -v shard="$shard" -v shards="$shards" '{
      best = 0
      for (i = 1; i < shards; i++) if (load[i] < load[best]) best = i
      load[best] += $1
      if (best == shard) print $2
    }'
}

mine=$(pick)
if [[ -z $mine ]]; then
  echo "shard $shard of $shards has nothing to run"
  exit 0
fi

case $kind in
  test)
    # shellcheck disable=SC2046
    go test -tags crdb_test -timeout 55m "$@" $(sed 's|^|./|' <<<"$mine")
    ;;
  logic)
    status=0
    for p in "${LOGIC_PKGS[@]}"; do
      tests=$(grep "^$p:" <<<"$mine" | cut -d: -f2 | paste -sd'|' -) || true
      [[ -z $tests ]] && continue
      go test -tags crdb_test -timeout 55m -v "$@" "./$p" -run "^($tests)\$" || status=1
    done
    exit $status
    ;;
esac
