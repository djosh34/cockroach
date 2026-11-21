# Migrating from CockroachDB v23.2

patchroaddb starts on the data directories of a stopped CockroachDB v23.2
cluster. The on-disk formats and the cluster version (23.2) are the same, so
the cluster keeps its data, users, grants and settings. The CI job
`migration` does this with the official v23.2.15 release on every pull
request; see [build/ci/migration.sh](../build/ci/migration.sh).

The whole cluster stops for the move. There are no mixed clusters of
CockroachDB and patchroaddb nodes, and no way back once patchroaddb has
started on the data.

## Not supported

- **Encrypted stores.** A store that CockroachDB encrypted at rest
  (`--enterprise-encryption`) does not open: patchroaddb fails with
  "encrypted stores are unsupported". A store on disaggregated shared
  storage fails too, with "shared storage is unsupported".
- **Versions other than v23.2.** Upgrade CockroachDB to v23.2.15 first, the
  version that CI tests, and let the upgrade finish: `SHOW CLUSTER SETTING
  version` must show `23.2`.
- **CockroachDB backups.** patchroaddb cannot read them. Take a new full
  backup after the move.

## What does not carry over

patchroaddb refuses to start while the cluster holds any of these. Drop them
in CockroachDB first:

- running, paused or reverting jobs of type BACKUP, RESTORE, CHANGEFEED,
  REPLICATION STREAM INGESTION or REPLICATION STREAM PRODUCER;
- backup and changefeed schedules;
- spatial columns (GEOMETRY, GEOGRAPHY, BOX2D), spatial indexes, and
  composite types or functions that use those types;
- multi-region databases, their region enums and table localities (REGIONAL
  BY ROW and the others);
- partitioned tables and indexes (PARTITION BY);
- PL/pgSQL functions and procedures;
- views, functions, column defaults, ON UPDATE and computed expressions,
  check and unique constraints, partial index predicates and TTL
  expressions that call a function patchroaddb removed, such as the spatial
  functions, `decrypt` and `crdb_internal.fips_ready`, or cast to a spatial
  type.

The check also counts dropped objects that the garbage collector has not
removed yet; the list marks them "dropped". Wait until it removes them, or
lower `gc.ttlseconds` on the table before you drop it.

These carry over but are not used:

- Finished jobs of those types stay as history: `SHOW JOBS` lists them until
  the job retention period removes them.
- Passwords and the host-based authentication setting. patchroaddb admits
  SQL clients only by client certificate; each user needs a client rule
  (see the README).
- The `cockroach cert` files. patchroaddb needs certificates from your own CA
  and a `--config` file; see the README.
- Cluster settings of removed features, which patchroaddb ignores, and the
  DB Console, which patchroaddb does not have.

## Steps

1. Make the certificates, the rules file and the `--config` file for every
   node, as the README describes. The node certificate needs the DNS names
   or IP addresses that clients connect to. Give every existing SQL user
   that logs in a client rule. With a backup section, the cluster takes its
   first full backup as soon as patchroaddb has started.
2. On the running CockroachDB cluster, cancel the jobs and drop the
   schedules and objects listed above. These queries find the jobs and
   schedules:

   ```sql
   SELECT job_id, job_type, status FROM [SHOW JOBS]
   WHERE job_type IN ('BACKUP', 'RESTORE', 'CHANGEFEED',
       'REPLICATION STREAM INGESTION', 'REPLICATION STREAM PRODUCER')
     AND status NOT IN ('succeeded', 'failed', 'canceled', 'revert-failed');

   SELECT schedule_id, schedule_name FROM system.scheduled_jobs
   WHERE executor_type IN ('scheduled-backup-executor', 'scheduled-changefeed-executor');
   ```

   `CANCEL JOB <id>` cancels a job; wait until its status is `canceled`.
   `DROP SCHEDULE <id>` drops a schedule.
3. Stop every node.
4. Start patchroaddb on every node with the same `--store`, `--join`,
   address and locality flags, plus `--config`. Drop the flags that
   patchroaddb does not have, such as `--certs-dir`.
5. If the cluster still holds something from the list above, each node
   prints what to drop and exits with an error, for example:

   ```
   ERROR: server startup failed: patchroaddb server exited with error: this cluster uses features that patchroaddb does not have:
     table shop.public.places: column "geom" has type GEOMETRY
     table shop.public.places: index "places_geom_idx" is a spatial index
     schedule 1229414376759263233 "nightly": runs scheduled-backup-executor
   ```

   A node that has lost quorum because the others exited may wait instead;
   stop it. Start the cluster with CockroachDB v23.2 again, drop what the
   list names, stop it and go back to step 4. patchroaddb checks before it
   runs any job or upgrade, so CockroachDB still opens the data.
