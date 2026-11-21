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
- creates no certificates and admits nodes and SQL clients only by TLS
  client certificates from your own CA, matched by exact CN and SPIFFE ID
  rules; there are no passwords;
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

## Certificates and identity

A secure node takes `--config <file>`, a YAML file that names PEM files:

```
cert: node.crt           # node cert, then its intermediates
key: node.key
node-ca: node-ca.crt     # CAs trusted for node-to-node connections
client-ca: client-ca.crt # CAs trusted for SQL and HTTP clients; may be the same file
rules: rules             # identity rules, below
client-cert: client.crt  # optional, used by the CLI for SQL
client-key: client.key
```

Relative paths are relative to the config file. A node cert needs both
server and client extended key usage, and the DNS names or IP addresses that
clients connect to, since SQL clients check them with `sslmode=verify-full`.
Nodes check each other by the node CA and the node rules, not by host name.

The rules file admits certificates by exact CN and SPIFFE ID (the cert's
only URI SAN). A line matches only if every key it lists matches:

```
node spiffe=spiffe://example.org/db/node
client cn=alice role=alice
client spiffe=spiffe://example.org/app role=app
client cn=ops spiffe=spiffe://example.org/ops role=root
```

The RPC port admits only node certs from the node CA that match a node line.
SQL admits client certs from the client CA; the user a client asks for must
be the role of a client line its cert matches. The HTTP API treats a node
cert as `root` and a client cert as the role of its first matching client
line; `/health` and `/_status/vars` need no cert.

A node reloads the files on SIGHUP and when anything changes in their
directories, so a Kubernetes secret update applies by itself. A new set
replaces the old one only if every file loads; otherwise the node logs the
error and counts it in `security.certificate.reload.failures`. Nodes share
their loaded rules: `patchroaddb debug gossip-values --config <file>` shows
each node's rules, and a node whose rules differ from a live peer's logs the
differing lines and sets `security.identity.rules.mismatched_nodes`.

The admin commands (`init`, `node`, `debug zip` and others) take the same
`--config`: they use the node cert for RPCs and the client cert for SQL.
`patchroaddb sql --config <file>` connects with the client cert. Without
`--user` it connects as the role of the first client rule that matches that
cert, and otherwise as root. A URL with `sslcert`, `sslkey` and
`sslrootcert` works too.

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
