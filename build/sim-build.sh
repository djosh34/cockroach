#!/usr/bin/env bash
# Builds the simulator's two binaries (pkg/sim/doc.go) into OUT, by default
# bin/sim:
#
#   sim.test        the native driver: TestCampaign, TestReplay, TestPlanted
#   sim-gosim.test  pkg/sim translated by gosim, in which TestSeed runs
#
# With PLANTED=1 the translated binary has the planted bugs
# (pkg/util/planted). Usage: [PLANTED=1] build/sim-build.sh [OUT]
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
out=${1:-$root/bin/sim}
mkdir -p "$out"
cd "$root"
go build -C tools/gosim -o "$root/.gosim/bin/gosim" github.com/jellevandenhooff/gosim/cmd/gosim
tags=()
if [[ -n ${PLANTED:-} ]]; then
  tags=(-tags planted)
fi
rm -rf .gosim/metatest
.gosim/bin/gosim build-tests "${tags[@]}" ./pkg/sim
built=(.gosim/metatest/*/*.test)
if [[ ${#built[@]} != 1 ]]; then
  echo "expected one translated test binary, found: ${built[*]}" >&2
  exit 1
fi
cp "${built[0]}" "$out/sim-gosim.test"
go test -c -o "$out/sim.test" ./pkg/sim
echo "built $out/sim.test and $out/sim-gosim.test"
