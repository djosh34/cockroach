#!/usr/bin/env bash
# Runs one part of the lint checks. CI runs the parts as parallel jobs,
# because the whole-tree tools each need several GB of memory.
#
# Usage: build/ci/lint.sh vet|roachvet|staticcheck|returncheck|rest
#
# vet runs go vet. The other parts run lint tests of pkg/testutils/lint:
# roachvet (go vet with the repository's analyzers and errcheck),
# staticcheck, returncheck, and all the other lint tests. The tests call
# tools built from the versions pinned in go.mod.
set -euo pipefail
cd "$(dirname "$0")/../.."

part=${1:?usage: build/ci/lint.sh vet|roachvet|staticcheck|returncheck|rest}

if [[ $part == vet ]]; then
  # unsafeptr runs in the roachvet part, which knows its exceptions.
  exec go vet -unsafeptr=false ./pkg/...
fi

bin=$PWD/bin/lint
mkdir -p "$bin"
go build -o "$bin" \
  github.com/cockroachdb/crlfmt \
  github.com/cockroachdb/returncheck \
  honnef.co/go/tools/cmd/staticcheck \
  ./pkg/cmd/roachvet
export PATH=$bin:$PATH

heavy='TestRoachVet|TestStaticCheck|TestReturnCheck'
case $part in
  roachvet) run=(-run 'TestLint/TestRoachVet$') ;;
  staticcheck) run=(-run 'TestLint/TestStaticCheck$') ;;
  returncheck) run=(-run 'TestLint/TestReturnCheck$') ;;
  rest) run=(-skip "TestLint/($heavy)\$") ;;
  *)
    echo "unknown part $part" >&2
    exit 1
    ;;
esac

go test -tags lint -timeout 50m "${run[@]}" ./pkg/testutils/lint/...
