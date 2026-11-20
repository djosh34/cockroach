#!/usr/bin/env bash
# Starts a node with default addresses in a network namespace that has only
# loopback, uses it for a minute, and fails if the node or the SQL client
# tried to connect anywhere but loopback or tried a DNS lookup. strace
# records every connect, sendto and sendmsg call, so a blocked attempt is
# seen too.
#
# Usage: build/ci/no-outbound.sh <cockroach binary>
# Needs root, unshare, ip, strace and curl.
set -euo pipefail

bin=$(realpath "${1:?usage: build/ci/no-outbound.sh <cockroach binary>}")
if [[ $EUID -ne 0 ]]; then
  echo "run as root: it creates a network namespace" >&2
  exit 1
fi
if [[ ${NO_OUTBOUND_NETNS:-} != 1 ]]; then
  exec env NO_OUTBOUND_NETNS=1 unshare --net "$0" "$@"
fi

ip link set lo up
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
addr=127.0.0.1:26257
trace() { strace -f -qq -e trace=connect,sendto,sendmsg -o "$@"; }
sql() { "$bin" sql --insecure --host="$addr" -e "$1"; }

# No listen or advertise address: the node listens on every interface and
# advertises the machine's host name, as a default start does.
trace "$work/node.trace" "$bin" start-single-node --insecure \
  --store="$work/store" --log-dir="$work/logs" --pid-file="$work/pid" &
strace_pid=$!

for _ in $(seq 60); do
  sql 'SELECT 1' >/dev/null 2>&1 && break
  sleep 1
done
# The client is traced the same way as the node. Its connection to the node
# shows that strace records connections here.
trace "$work/client.trace" "$bin" sql --insecure --host="$addr" \
  -e 'CREATE TABLE t (k INT PRIMARY KEY); INSERT INTO t VALUES (1); SELECT * FROM t'
curl -sSf 'http://127.0.0.1:8080/health?ready=1' >/dev/null
# Background work such as diagnostics reporting and the update check used to
# start within seconds of node start.
sleep 60

# Stop the node; a graceful shutdown may take long, so kill it after 30s.
# strace exits with the node's status: 0 after a clean shutdown, 137 if it
# was killed. Anything else means the node or strace failed.
node=$(cat "$work/pid")
kill -TERM "$node"
for _ in $(seq 30); do
  kill -0 "$node" 2>/dev/null || break
  sleep 1
done
killed=false
kill -KILL "$node" 2>/dev/null && killed=true
status=0
wait "$strace_pid" || status=$?
if [[ $status -ne 0 && ! ($status -eq 137 && $killed == true) ]]; then
  echo "the traced node exited with status $status" >&2
  exit 1
fi

ip_calls='sa_family=AF_INET6?'
loopback='inet_addr\("127\.[0-9.]+"\)|inet_pton\(AF_INET6, "(::1|::ffff:127\.[0-9.]+)"'
for f in client.trace node.trace; do
  if [[ ! -r $work/$f ]]; then
    echo "strace wrote no $f; the check does not work" >&2
    exit 1
  fi
done
if ! grep -qE "$ip_calls.*($loopback)" "$work/client.trace"; then
  echo "strace did not record the client's connection; the check does not work" >&2
  exit 1
fi
# grep exits 1 when nothing matches; any other failure stops the script.
matches() { grep -E "$@" || [[ $? -eq 1 ]]; }
calls=$(matches "$ip_calls" "$work/node.trace")
echo "$(printf '%s' "$calls" | grep -c . || true) IP connection attempts by the node"
calls+=$'\n'$(matches "$ip_calls" "$work/client.trace")
bad=$(printf '%s\n' "$calls" | matches -v "$loopback" | matches .)
dns=$(matches 'htons\(53\)' "$work/node.trace" "$work/client.trace")
if [[ -n $bad || -n $dns ]]; then
  echo "the node or the client tried to reach the network:" >&2
  printf '%s\n' "$bad" "$dns" | sort -u >&2
  exit 1
fi
echo "no connection attempt beyond loopback"
