# patchroaddb

patchroaddb is a distributed SQL database. It is a fork of CockroachDB
v23.2.15, licensed under the Apache License, Version 2.0.

It keeps CockroachDB's core: a transactional, strongly consistent key-value
store with SQL on top, spoken over the PostgreSQL wire protocol. Compared with
CockroachDB v23.2.15 it:

- contains no code under the CockroachDB Community License and no license
  checks;
- builds with plain Go, without cgo, for Linux on amd64 and arm64;
- has no web UI, no spatial types, and none of CockroachDB's calls home
  (diagnostics reporting, update checks, crash reports);
- runs every transaction as SERIALIZABLE; READ COMMITTED and SNAPSHOT are
  accepted as syntax only;
- carries security and data-loss fixes made after the fork.

On-disk formats are CockroachDB 23.2's. Environment variables (`COCKROACH_*`),
the default data directory (`cockroach-data`), and SQL names such as
`crdb_internal` keep their CockroachDB names.

## Install

Each release on GitHub has a binary tarball per architecture,
`patchroaddb-<version>.linux-amd64.tgz` and
`patchroaddb-<version>.linux-arm64.tgz`, with their checksums in
`SHA256SUMS`.

The container image is `ghcr.io/djosh34/patchroaddb`, for linux/amd64 and
linux/arm64, tagged with each release version and `latest`:

```
docker run --rm ghcr.io/djosh34/patchroaddb version
```

## Use

```
patchroaddb start-single-node --insecure
patchroaddb sql --insecure
```

`patchroaddb --help` lists the commands. The
[CockroachDB v23.2 documentation](https://www.cockroachlabs.com/docs/v23.2/)
describes SQL and operations, apart from the features removed above.

## Build from source

You need Go; the `go` command fetches the version that `go.mod` names.

```
CGO_ENABLED=0 go build -o patchroaddb ./pkg/cmd/patchroaddb
```

[build/README.md](build/README.md) explains how to regenerate code, run the
tests and make a release.

## License

patchroaddb is licensed under the Apache License, Version 2.0 (see
[LICENSE](LICENSE) and [licenses/APL.txt](licenses/APL.txt)). It is based on
CockroachDB v23.2.15 by The Cockroach Authors, which was licensed under the
Business Source License 1.1 and became Apache 2.0 on its Change Date,
2026-10-01; [NOTICE](NOTICE) has the details. Third-party code keeps its own
license; see the [licenses](licenses) folder.

CockroachDB is a trademark of Cockroach Labs. patchroaddb is not affiliated
with or endorsed by Cockroach Labs.
