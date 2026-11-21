#!/usr/bin/env bash
# Runs the backup tests against a real S3 server: versitygw in a container.
# S3 is never mocked.
#
# Usage: build/ci/s3.sh [go test flags...]
set -euo pipefail
cd "$(dirname "$0")/../.."

# versity/versitygw:v1.10.0
image=versity/versitygw@sha256:89cad13cc981986ee284cbafea4613f9533ce6fd5f37a2c96c4c4fb8adb3a316
port=${S3_PORT:-7070}

export AWS_ACCESS_KEY_ID=patchroaddb-test
export AWS_SECRET_ACCESS_KEY=patchroaddb-test-secret
export PATCHROADDB_TEST_S3_ENDPOINT=http://127.0.0.1:$port

name=patchroaddb-s3-$$
docker run -d --rm --name "$name" -p "127.0.0.1:$port:7070" \
  -e ROOT_ACCESS_KEY="$AWS_ACCESS_KEY_ID" -e ROOT_SECRET_KEY="$AWS_SECRET_ACCESS_KEY" \
  --tmpfs /data "$image" --port :7070 posix /data >/dev/null
trap 'docker stop "$name" >/dev/null' EXIT
# Any HTTP answer means the server is up; it refuses unsigned requests.
for i in $(seq 30); do
  code=$(curl -s -o /dev/null -w '%{http_code}' "$PATCHROADDB_TEST_S3_ENDPOINT/" || true)
  [[ $code != 000 ]] && break
  if [[ $i == 30 ]]; then
    echo "the S3 server did not start" >&2
    docker logs "$name" >&2
    exit 1
  fi
  sleep 1
done

go test -tags crdb_test -timeout 25m "$@" ./pkg/backup/...
