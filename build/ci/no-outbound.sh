#!/usr/bin/env bash
# Starts a node in a network namespace that has only loopback, uses it for a
# minute, and fails if the node tried to connect anywhere but loopback or
# tried a DNS lookup. strace records every connect, sendto and sendmsg call,
# so a blocked attempt is seen too.
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

trace "$work/node.trace" "$bin" start-single-node --insecure \
  --store="$work/store" --log-dir="$work/logs" --pid-file="$work/pid" \
  --listen-addr="$addr" --http-addr=127.0.0.1:8080 &
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
node=$(cat "$work/pid")
kill -TERM "$node"
for _ in $(seq 30); do
  kill -0 "$node" 2>/dev/null || break
  sleep 1
done
kill -KILL "$node" 2>/dev/null || true
wait "$strace_pid" || true

ip_calls='sa_family=AF_INET6?'
loopback='inet_addr\("127\.|inet_pton\(AF_INET6, "(::1|::ffff:127\.)'
if ! grep -qE "$ip_calls.*$loopback" "$work/client.trace"; then
  echo "strace did not record the client's connection; the check does not work" >&2
  exit 1
fi
echo "$(grep -cE "$ip_calls" "$work/node.trace" || true) IP connection attempts by the node"
bad=$(grep -E "$ip_calls" "$work/node.trace" | grep -vE "$loopback" || true)
dns=$(grep -E 'htons\(53\)' "$work/node.trace" || true)
if [[ -n $bad || -n $dns ]]; then
  echo "the node tried to reach the network:" >&2
  printf '%s\n' "$bad" "$dns" | sort -u >&2
  exit 1
fi
echo "no connection attempt beyond loopback"
