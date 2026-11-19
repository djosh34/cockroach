#!/usr/bin/env bash
# Runs the lint tests of pkg/testutils/lint. They call these tools, which are
# built from the versions pinned in go.mod.
set -euo pipefail
cd "$(dirname "$0")/../.."

bin=$PWD/bin/lint
mkdir -p "$bin"
go build -o "$bin" \
  github.com/cockroachdb/crlfmt \
  github.com/cockroachdb/returncheck \
  github.com/kisielk/errcheck \
  honnef.co/go/tools/cmd/staticcheck \
  ./pkg/cmd/roachvet
export PATH=$bin:$PATH

go test -tags lint -timeout 60m ./pkg/testutils/lint/...
