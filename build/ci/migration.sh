#!/usr/bin/env bash
# Migrates a 3-node CockroachDB v23.2.15 cluster to patchroaddb in place, as
# docs/migration.md describes, and checks the result:
#
#  1. The official v23.2.15 binary runs a 3-node cluster with tables,
#     indexes (forward and inverted indexes on an array and a JSON column
#     among them), a view, a
#     sequence, a type, a function, users, grants and rows. The cluster also
#     gets two things that patchroaddb cut: a spatial column with a spatial
#     index, and a backup schedule.
#  2. patchroaddb must refuse to start on those data directories and list
#     both.
#  3. v23.2.15 drops them, records what every object and row looks like,
#     and stops.
#  4. patchroaddb starts on the same directories with certs from a new CA and
#     identity rules, shows the same objects and rows, reads and updates rows
#     through each array and JSON index that upstream built, and takes new
#     tables and writes.
#
# Usage: build/ci/migration.sh <patchroaddb binary>
# Set UPSTREAM_TARBALL to a downloaded release tarball to skip the download;
# its checksum is still checked. Needs curl, openssl and sha256sum.
set -euo pipefail

version=v23.2.15
# From https://binaries.cockroachdb.com/cockroach-v23.2.15.linux-<arch>.tgz.sha256sum
declare -A sha256=(
  [amd64]=3ed9f7912a2b40272b6410507102d18f84e6ed83c3039a7ae21a063654198a56
  [arm64]=94f10d899b7f47d45c1a36a31751f4eb0e3c0a9c34e857bd5d065672aec888d2
)

new=$(realpath "${1:?usage: build/ci/migration.sh <patchroaddb binary>}")
case $(uname -m) in
  x86_64) arch=amd64 ;;
  aarch64) arch=arm64 ;;
  *) echo "unsupported machine $(uname -m)" >&2; exit 1 ;;
esac

work=$(mktemp -d)
pids=()
# On failure, show the end of each node's output and log, so that CI has the
# cause.
cleanup() {
  local status=$?
  if [[ $status -ne 0 ]]; then
    for f in "$work"/*.out "$work"/n*/logs/cockroach.log; do
      [[ -f $f ]] || continue
      echo "==== $f" >&2
      tail -n 50 "$f" >&2
    done
  fi
  for pid in "${pids[@]}"; do kill -KILL "$pid" 2>/dev/null || true; done
  rm -rf "$work"
}
trap cleanup EXIT

tarball=cockroach-$version.linux-$arch.tgz
if [[ -n ${UPSTREAM_TARBALL:-} ]]; then
  cp "$UPSTREAM_TARBALL" "$work/$tarball"
else
  curl -sSfL -o "$work/$tarball" "https://binaries.cockroachdb.com/$tarball"
fi
echo "${sha256[$arch]}  $work/$tarball" | sha256sum -c -
tar -C "$work" -xzf "$work/$tarball"
old=$work/cockroach-$version.linux-$arch/cockroach

join=127.0.0.1:26257,127.0.0.1:26258,127.0.0.1:26259

# start <name> <binary> <flags...> starts nodes 1 to 3 in the background,
# with their output in $work/<name>-n<i>.out.
start() {
  local name=$1 bin=$2
  shift 2
  pids=()
  for i in 1 2 3; do
    "$bin" start "$@" --store="$work/n$i" --join="$join" \
      --listen-addr="127.0.0.1:$((26256 + i))" --http-addr="127.0.0.1:$((8079 + i))" \
      >"$work/$name-n$i.out" 2>&1 &
    pids+=($!)
  done
}

# stop stops the nodes that start started and waits for them to exit. The
# last node loses quorum while it drains and may wait forever, so a node that
# still runs after 30 seconds is killed. Committed writes are already synced
# to disk, so this loses nothing.
stop() {
  local pid
  for signal in TERM KILL; do
    kill -"$signal" "${pids[@]}" 2>/dev/null || true
    for _ in $(seq 30); do
      local alive=false
      for pid in "${pids[@]}"; do kill -0 "$pid" 2>/dev/null && alive=true; done
      [[ $alive == false ]] && break
      sleep 1
    done
  done
  for pid in "${pids[@]}"; do
    if kill -0 "$pid" 2>/dev/null; then
      echo "node $pid did not stop" >&2
      exit 1
    fi
    wait "$pid" || true
  done
  pids=()
}

# wait_sql <sql command...> waits until the cluster answers a query.
wait_sql() {
  for _ in $(seq 120); do
    "$@" -e 'SELECT 1' >/dev/null 2>&1 && return
    sleep 1
  done
  echo "the cluster did not answer within 2 minutes" >&2
  exit 1
}

old_sql() { "$old" sql --insecure --host=127.0.0.1:26257 --format=tsv "$@"; }

echo "== v23.2.15: create a cluster with data"
start old "$old" --insecure
"$old" init --insecure --host=127.0.0.1:26257
wait_sql old_sql
old_sql <<'SQL'
CREATE DATABASE shop;
SET DATABASE = shop;
CREATE TYPE status AS ENUM ('new', 'paid', 'sent');
CREATE TABLE customers (
  id INT PRIMARY KEY,
  name STRING NOT NULL,
  email STRING UNIQUE,
  created TIMESTAMPTZ NOT NULL DEFAULT now(),
  INDEX (name)
);
CREATE SEQUENCE order_ids START 1000;
CREATE TABLE orders (
  id INT PRIMARY KEY DEFAULT nextval('order_ids'),
  customer INT NOT NULL REFERENCES customers (id),
  total DECIMAL(10, 2) NOT NULL CHECK (total >= 0),
  status status NOT NULL DEFAULT 'new',
  tags STRING[],
  details JSONB,
  label STRING AS (lower(status::STRING) || ':' || id::STRING) STORED,
  INDEX (customer),
  INDEX tags_fwd (tags),
  INDEX details_fwd (details),
  INVERTED INDEX tags_idx (tags),
  INVERTED INDEX details_idx (details)
);
CREATE VIEW big_orders AS SELECT id, customer, total FROM orders WHERE total > 100;
CREATE FUNCTION double_total(x DECIMAL) RETURNS DECIMAL LANGUAGE SQL AS 'SELECT x * 2';
INSERT INTO customers (id, name, email, created)
  SELECT i, 'customer ' || i::STRING, 'c' || i::STRING || '@example.com',
    '2024-01-01'::TIMESTAMPTZ + i * INTERVAL '1 hour'
  FROM generate_series(1, 1000) AS g(i);
INSERT INTO orders (customer, total, status, tags, details)
  SELECT (i % 1000) + 1, (i % 500)::DECIMAL / 4, (ARRAY['new', 'paid', 'sent'])[(i % 3) + 1]::status,
    ARRAY['tag' || (i % 7)::STRING, 'all'], json_build_object('color', 'c' || (i % 5)::STRING, 'n', i)
  FROM generate_series(1, 5000) AS g(i);
ALTER TABLE orders SPLIT AT VALUES (2000), (3000), (4000);
CREATE USER alice;
CREATE ROLE readers;
GRANT readers TO alice;
GRANT SELECT ON big_orders TO readers;
GRANT CONNECT ON DATABASE shop TO readers;

-- Cut features, which patchroaddb must refuse to start on.
CREATE TABLE places (id INT PRIMARY KEY, name STRING, geom GEOMETRY, INVERTED INDEX places_geom_idx (geom));
INSERT INTO places VALUES (1, 'origin', 'POINT(0 0)'), (2, 'one', 'POINT(1 1)');
CREATE SCHEDULE nightly FOR BACKUP DATABASE shop INTO 'nodelocal://1/backups'
  RECURRING '@daily' WITH SCHEDULE OPTIONS first_run = '2099-01-01';
-- A finished backup job, which stays as history.
BACKUP DATABASE shop INTO 'nodelocal://1/backups';
SQL
stop

echo "== patchroaddb: refuse to start on the cut features"
# Certs from a new CA; patchroaddb creates none itself.
certs=$work/certs
mkdir -p "$certs"
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 30 \
  -subj "/CN=migration test CA" -keyout "$certs/ca.key" -out "$certs/ca.crt" 2>/dev/null
# cert <name> <subject> <extensions> makes <name>.crt and <name>.key, signed by the CA.
cert() {
  openssl req -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -subj "$2" \
    -keyout "$certs/$1.key" -out "$certs/$1.csr" 2>/dev/null
  openssl x509 -req -in "$certs/$1.csr" -CA "$certs/ca.crt" -CAkey "$certs/ca.key" \
    -CAcreateserial -days 30 -extfile <(printf '%s\n' "$3") -out "$certs/$1.crt" 2>/dev/null
}
cert node "/CN=node" "subjectAltName=URI:spiffe://migration.test/node,DNS:localhost,IP:127.0.0.1
extendedKeyUsage=serverAuth,clientAuth"
cert root "/CN=root" "extendedKeyUsage=clientAuth"
cert alice "/CN=alice" "extendedKeyUsage=clientAuth"
cat >"$certs/rules" <<'EOF'
node spiffe=spiffe://migration.test/node
client cn=root role=root
client cn=alice role=alice
EOF
cat >"$certs/config.yaml" <<'EOF'
cert: node.crt
key: node.key
node-ca: ca.crt
client-ca: ca.crt
rules: rules
client-cert: root.crt
client-key: root.key
EOF
new_sql() { "$new" sql --config="$certs/config.yaml" --host=127.0.0.1:26257 --format=tsv "$@"; }

start refuse "$new" --config="$certs/config.yaml"
# Every node that reads the catalog refuses. Once two have exited, the last
# one has no quorum and may wait forever, so stop it after that.
exited=0
for _ in $(seq 180); do
  exited=0
  for pid in "${pids[@]}"; do kill -0 "$pid" 2>/dev/null || exited=$((exited + 1)); done
  [[ $exited -ge 2 ]] && break
  sleep 1
done
if [[ $exited -lt 2 ]]; then
  echo "patchroaddb did not refuse to start within 3 minutes" >&2
  exit 1
fi
refused=0
for i in 1 2 3; do
  pid=${pids[$((i - 1))]}
  kill -0 "$pid" 2>/dev/null && continue
  status=0
  wait "$pid" || status=$?
  out=$work/refuse-n$i.out
  if [[ $status -eq 0 ]]; then
    echo "node $i exited with status 0 on data with cut features" >&2
    exit 1
  fi
  for want in 'this cluster uses features that patchroaddb does not have' \
    'table shop.public.places: column "geom" has type GEOMETRY' \
    'table shop.public.places: index "places_geom_idx" is a spatial index' \
    '"nightly": runs scheduled-backup-executor'; do
    if ! grep -qF "$want" "$out"; then
      echo "node $i did not report: $want" >&2
      exit 1
    fi
  done
  refused=$((refused + 1))
done
echo "$refused nodes refused, exit statuses non-zero"
stop

echo "== v23.2.15: drop the cut features and record the data"
start old "$old" --insecure
wait_sql old_sql
old_sql -e "
ALTER TABLE shop.places DROP COLUMN geom;
DROP SCHEDULES SELECT schedule_id FROM system.scheduled_jobs WHERE executor_type = 'scheduled-backup-executor';"

# The same queries run on both binaries; their output must be the same.
cat >"$work/check.sql" <<'SQL'
SELECT database_name, owner FROM [SHOW DATABASES] ORDER BY 1;
SET DATABASE = shop;
SHOW CREATE ALL TABLES;
SHOW CREATE ALL TYPES;
SELECT create_statement FROM [SHOW CREATE FUNCTION double_total];
SELECT username, member_of FROM [SHOW USERS] ORDER BY 1;
SHOW GRANTS ON DATABASE shop;
SHOW GRANTS ON TABLE *;
SELECT * FROM customers ORDER BY id;
SELECT * FROM orders ORDER BY id;
SELECT * FROM big_orders ORDER BY id;
SELECT * FROM places ORDER BY id;
SELECT double_total(total) FROM orders WHERE id = 1500;
-- A hash of every index entry. It cannot hash the inverted indexes of
-- orders; the queries after the migration read through those.
SHOW EXPERIMENTAL_FINGERPRINTS FROM TABLE customers;
SQL
old_sql <"$work/check.sql" >"$work/want.txt"
stop

echo "== patchroaddb: start on the same data directories"
start new "$new" --config="$certs/config.yaml"
wait_sql new_sql
new_sql <"$work/check.sql" >"$work/got.txt"
if ! diff -u "$work/want.txt" "$work/got.txt"; then
  echo "patchroaddb shows different objects or rows than v23.2.15 did" >&2
  exit 1
fi
echo "every object and row is the same ($(wc -l <"$work/got.txt") lines of output)"

# expect <want> <sql> runs a query on patchroaddb and compares its output.
expect() {
  local got
  got=$(new_sql -e "$2" | tail -n +2)
  if [[ $got != "$1" ]]; then
    printf 'query: %s\nwant: %s\ngot:  %s\n' "$2" "$1" "$got" >&2
    exit 1
  fi
}

echo "== patchroaddb: the finished backup job is still listed"
expect $'BACKUP\tsucceeded' "SELECT job_type, status FROM [SHOW JOBS] WHERE job_type = 'BACKUP'"

echo "== patchroaddb: read and update rows through the indexes upstream built"
# Forward indexes hold whole array and JSON values in their keys.
tag5=$(new_sql -e "SELECT count(*) FROM shop.orders@primary WHERE tags = ARRAY['tag5', 'all']" | tail -n 1)
expect "$tag5" "SELECT count(*) FROM shop.orders@tags_fwd WHERE tags = ARRAY['tag5', 'all']"
new_sql -e "UPDATE shop.orders@tags_fwd SET tags = ARRAY['tag5', 'fwd'] WHERE tags = ARRAY['tag5', 'all']" >/dev/null
expect "$tag5" "SELECT count(*) FROM shop.orders@tags_fwd WHERE tags = ARRAY['tag5', 'fwd']"
expect "0" "SELECT count(*) FROM shop.orders@tags_fwd WHERE tags = ARRAY['tag5', 'all']"
expect "$tag5" "SELECT count(*) FROM shop.orders@primary WHERE tags = ARRAY['tag5', 'fwd']"

expect "1234" "SELECT details->>'n' FROM shop.orders@details_fwd WHERE details = '{\"color\": \"c4\", \"n\": 1234}'"
new_sql -e "UPDATE shop.orders@details_fwd SET details = '{\"color\": \"c4\", \"n\": 1234, \"fwd\": true}' WHERE details = '{\"color\": \"c4\", \"n\": 1234}'" >/dev/null
expect "1234" "SELECT details->>'n' FROM shop.orders@details_fwd WHERE details = '{\"color\": \"c4\", \"n\": 1234, \"fwd\": true}'"
expect "0" "SELECT count(*) FROM shop.orders@details_fwd WHERE details = '{\"color\": \"c4\", \"n\": 1234}'"

# Inverted indexes hold each array element and JSON path.
tagged=$(new_sql -e "SELECT count(*) FROM shop.orders@primary WHERE 'tag3' = ANY tags" | tail -n 1)
expect "$tagged" "SELECT count(*) FROM shop.orders@tags_idx WHERE tags @> ARRAY['tag3']"
new_sql -e "UPDATE shop.orders@tags_idx SET tags = array_append(tags, 'moved') WHERE tags @> ARRAY['tag3']" >/dev/null
expect "$tagged" "SELECT count(*) FROM shop.orders@tags_idx WHERE tags @> ARRAY['moved']"
expect "$tagged" "SELECT count(*) FROM shop.orders@primary WHERE 'moved' = ANY tags"

colored=$(new_sql -e "SELECT count(*) FROM shop.orders@primary WHERE details->>'color' = 'c2'" | tail -n 1)
expect "$colored" "SELECT count(*) FROM shop.orders@details_idx WHERE details @> '{\"color\": \"c2\"}'"
new_sql -e "UPDATE shop.orders@details_idx SET details = details || '{\"moved\": true}' WHERE details @> '{\"color\": \"c2\"}'" >/dev/null
expect "$colored" "SELECT count(*) FROM shop.orders@details_idx WHERE details @> '{\"moved\": true}'"
expect "$colored" "SELECT count(*) FROM shop.orders@primary WHERE details->>'moved' = 'true'"

echo "== patchroaddb: new tables and writes"
last=$(new_sql -e "SELECT max(id) FROM shop.orders" | tail -n 1)
new_sql -e "
INSERT INTO shop.orders (customer, total) VALUES (1, 9.99);
CREATE TABLE shop.notes (id INT PRIMARY KEY, body STRING);
INSERT INTO shop.notes VALUES (1, 'after the migration');" >/dev/null
expect "1" "SELECT count(*) FROM shop.orders WHERE id > $last"
expect "after the migration" "SELECT body FROM shop.notes"

echo "== patchroaddb: alice logs in with a client cert and keeps her grants"
alice_url="postgresql://alice@127.0.0.1:26257/shop?sslmode=verify-full&sslrootcert=$certs/ca.crt&sslcert=$certs/alice.crt&sslkey=$certs/alice.key"
got=$("$new" sql --url="$alice_url" --format=tsv -e "SELECT count(*) FROM big_orders" | tail -n 1)
want=$(new_sql -e "SELECT count(*) FROM shop.big_orders" | tail -n 1)
if [[ $got != "$want" ]]; then
  echo "alice sees $got rows of big_orders, want $want" >&2
  exit 1
fi
denied=$("$new" sql --url="$alice_url" -e "SELECT count(*) FROM customers" 2>&1 || true)
if [[ $denied != *"user alice does not have SELECT privilege on relation customers"* ]]; then
  printf 'alice read customers without a grant, or failed for another reason:\n%s\n' "$denied" >&2
  exit 1
fi
stop
echo "migration test passed"
