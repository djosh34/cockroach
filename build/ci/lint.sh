#!/usr/bin/env bash
# Runs one part of the lint checks. CI runs the parts as parallel jobs,
# because the whole-tree tools each need several GB of memory.
#
# Usage: build/ci/lint.sh vet|roachvet[-K-N]|staticcheck[-K-N]|returncheck|rest
#
# vet runs go vet. The other parts run lint tests of pkg/testutils/lint:
# roachvet (go vet with the repository's analyzers and errcheck),
# staticcheck, returncheck, and all the other lint tests. roachvet-K-N runs
# part K of N of the analyzers, and staticcheck-K-N checks part K of N of the
# packages (every Nth one, starting at K).
# The tests call tools built from the versions pinned in go.mod.
set -euo pipefail
cd "$(dirname "$0")/../.."

part=${1:?usage: build/ci/lint.sh vet|roachvet[-K-N]|staticcheck[-K-N]|returncheck|rest}

if [[ $part == vet ]]; then
  # unsafeptr runs in the roachvet part, which knows its exceptions.
  exec go vet -unsafeptr=false ./pkg/...
fi

bin=$PWD/bin/lint
mkdir -p "$bin"
go build -o "$bin" \
  github.com/cockroachdb/crlfmt \
  honnef.co/go/tools/cmd/staticcheck \
  ./pkg/cmd/returncheck \
  ./pkg/cmd/roachvet \
  ./pkg/sql/opt/optgen/cmd/optfmt
export PATH=$bin:$PATH

heavy='TestRoachVet|TestStaticCheck|TestReturnCheck'
case $part in
  roachvet) run=(-run 'TestLint/TestRoachVet$') ;;
  roachvet-*-*)
    IFS=- read -r _ k n <<<"$part"
    export ROACHVET_PART=$k/$n
    # go vet caches results by tool and flags, not by environment, so each
    # part needs its own build cache.
    GOCACHE="$(go env GOCACHE)-roachvet-$k-$n"
    export GOCACHE
    run=(-run 'TestLint/TestRoachVet$')
    ;;
  staticcheck) run=(-run 'TestLint/TestStaticCheck$') ;;
  staticcheck-*-*)
    IFS=- read -r _ k n <<<"$part"
    STATICCHECK_PKGS=$(go list ./pkg/... | awk -v k="$k" -v n="$n" '(NR - 1) % n == k')
    export STATICCHECK_PKGS
    run=(-run 'TestLint/TestStaticCheck$')
    ;;
  returncheck) run=(-run 'TestLint/TestReturnCheck$') ;;
  rest) run=(-skip "TestLint/($heavy)\$") ;;
  *)
    echo "unknown part $part" >&2
    exit 1
    ;;
esac

go test -tags lint -timeout 50m "${run[@]}" ./pkg/testutils/lint/...
