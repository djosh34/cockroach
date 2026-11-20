# Building, generating and testing

Everything uses the plain `go` command. `go.mod` names the Go toolchain with its
`toolchain` line; with the default `GOTOOLCHAIN=auto`, `go` downloads that
version when the installed one is older. Builds target Linux on amd64 and
arm64, without cgo.

## Build

```
CGO_ENABLED=0 go build -o patchroaddb ./pkg/cmd/patchroaddb
```

## Generated code

Generated code is checked in. After changing a `.proto`, `.y`, `.opt` or
template file, or anything else a generator reads, run:

```
build/generate.sh            # all steps
build/generate.sh proto      # only some steps, in the order given
```

The steps are listed at the top of the script. The Go tools it uses are pinned
by the `tool` lines in `go.mod`; `protoc` is downloaded into `bin/` and checked
against a pinned checksum. CI fails if running the script changes any file.

## Tests

```
go test -tags crdb_test ./pkg/sql/parser
```

Tests run with the `crdb_test` build tag, like in CI. Logic tests are Go tests
too, generated per config (`local` and `fakedist`):

```
go test -tags crdb_test ./pkg/sql/logictest/tests/local -run 'TestLogic_select$'
```

The lint tests need the lint tools on the `PATH`. `build/ci/lint.sh` builds
them and runs one part of the checks: `vet`, `roachvet` (go vet with the
repository's analyzers and errcheck; `roachvet-K-N` runs part K of N of the
analyzers), `staticcheck` (`staticcheck-K-N` checks part K of N of the
packages), `returncheck` or `rest`.

## CI

`.github/workflows/ci.yml` runs on every pull request and push to `main`:

- `build`: `go build ./...` without cgo for linux/amd64 and linux/arm64.
- `generate`: `build/generate.sh`, failing on any change.
- `lint`: `go vet` and the lint tests, in seven parallel parts, because the
  whole-tree tools each need several GB of memory.
- `test` and `logic`: the tests and the logic tests, in shards. `build/ci/test.sh`
  picks the tests of a shard. It balances the shards by the times in
  `build/ci/times.txt`, which come from earlier CI runs, and splits the
  slowest packages into parts that each run every Nth test function.
- `ci-ok`: passes only if every other job passed. It is the check that `main`
  requires.

`.github/workflows/nightly.yml` runs the same shards with the race detector,
every night and on manual start.

## Release

The version is in `pkg/build/version.txt`. `build/release.sh <tag> <out dir>`
builds the release files: a `patchroaddb-<tag>.linux-<arch>.tgz` for amd64
and arm64 with the binary and the license files, `SHA256SUMS`, and the build
context of the image. The tag must equal `version.txt`.

`.github/workflows/release.yml` runs it. On a pushed `v*` tag it puts the
tarballs and `SHA256SUMS` on a GitHub release and pushes the multi-arch image
`ghcr.io/djosh34/patchroaddb` tagged with the tag and `latest`. Only its
`publish` job has write permissions. Started by hand, it builds and checks
everything and publishes nothing.

`build/deploy` holds the Dockerfile of the image and its entry point script.
