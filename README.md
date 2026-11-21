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
- carries security and data-loss fixes made after the fork;
- backs itself up to S3, with full backups and a continuous archive, and
  restores a whole cluster to any time in that window, like pgBackRest.

On-disk formats are CockroachDB 23.2's. Environment variables (`COCKROACH_*`),
the default data directory (`cockroach-data`), and SQL names such as
`crdb_internal` keep their CockroachDB names. A stopped CockroachDB v23.2
cluster can move to patchroaddb in place;
[docs/migration.md](docs/migration.md) has the steps and what does not
carry over.

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

## Backups and point-in-time restore

A cluster backs itself up to S3 when the `--config` file has a backup
section:

```
backup:
  bucket: backups
  prefix: prod                       # optional: the repository's folder
  endpoint: https://s3.example.com   # optional: an S3-compatible server
  region: us-east-1                  # optional
  full-schedule: "0 3 * * *"         # cron, UTC; "@every 6h" works too
  archive-interval: 60s              # optional, default 60s
  keep-full: 2                       # full backups to keep
```

An insecure node reads only the backup section of `--config`. S3
credentials come from each node's environment through the AWS SDK's
default chain (`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`, a profile, or
an instance role). With an `endpoint` the client uses path-style requests.
Every node should have the same section: nodes share it, and a node whose
section differs from a live peer's logs the difference and sets
`backup.config.mismatched_nodes`.

It works like pgBackRest. One job, `AUTO BACKUP` in
`SHOW AUTOMATIC JOBS`, takes a full backup when the schedule says, and every
archive interval uploads an archive segment: one object with every change
since the previous segment. The repository holds:

```
repository.json                    the cluster that owns it
full/<time>/000001.sst.gz ...      the files of a full backup
full/<time>/backup.manifest        written last; the backup exists once it does
archive/0000000000000001.tar.gz    archive segments 1, 2, 3, ...
```

Files are gzip, not encrypted. A segment exists only once its upload has
finished; a failed upload is made again, from where the last segment ended.
The cluster keeps the history that the next segment needs from garbage
collection until it is uploaded. After each full backup the job deletes the
full backups beyond `keep-full` and the segments they alone needed. Tables
marked `exclude_data_from_backup` are backed up without their rows.

The archive interval is how often a segment is made, not a bound on what
you can lose: if every node is lost, you can restore up to the end of the
last uploaded segment, and an upload can take longer than the interval or
wait for S3. `patchroaddb backup info` shows that time.

A repository belongs to the cluster that first wrote into it. Any other
cluster, such as one restored with a copy of the config, refuses to write
into it or expire it, and its `AUTO BACKUP` job says so; give it a section
with another bucket or prefix.

The `backup` commands read the repository from `--config`, with the
credentials of their environment:

- `patchroaddb backup info --config <file>`: the full backups, the archive
  and the last restorable time.
- `patchroaddb backup check --config <file> [--backup <name>]`: reads a full
  backup, the last one by default, and every segment after it, and checks
  their checksums and that the segments follow each other.
- `patchroaddb backup expire --config <file>`: deletes what `keep-full` no
  longer keeps, as the cluster does after each full backup.
- `patchroaddb backup restore --config <file> --host <node> [--target <time>]`:
  restores the whole cluster into a new cluster, as of `--target` (RFC 3339,
  or the decimal that `info` and `cluster_logical_timestamp()` show) or the
  last restorable time.

A restore needs a new, empty cluster, started like any other, in which no
table was ever created; it may have its own backup section. If a restore
fails after it has started writing, that cluster takes no other restore;
start a new one. Its nodes need S3 credentials for the source repository.
The restore takes the last full backup at or before the target and every
segment after it up to the target, and stops at the first missing or broken
segment; a target after the last restorable time is refused. Like
CockroachDB's full cluster restore, it brings back all databases, schemas,
tables, types, functions and views with their data and IDs, and the users,
roles, grants, cluster settings, zone configurations, comments and table
statistics. defaultdb and postgres are replaced by the backup's. Tables
marked `exclude_data_from_backup` come back empty. Jobs and schedules are
not restored, and nothing about backups is; tables with row-level TTL get
new TTL schedules, and schema changes that were running at the target time
get new jobs. A CREATE TABLE AS or materialized view that was still being
created is left out. Restored tables keep no history from before the
restore.

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
