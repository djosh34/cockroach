#!/usr/bin/env bash
# Builds the release files of patchroaddb into <out dir>:
#
#   patchroaddb-<tag>.linux-amd64.tgz
#   patchroaddb-<tag>.linux-arm64.tgz
#   SHA256SUMS
#   image/      the build context of the release image (build/deploy/Dockerfile)
#
# <tag> must equal pkg/build/version.txt, so the binary, the files and the
# image all carry the one patchroaddb version.
#
# Usage: build/release.sh <tag> <out dir>
set -euo pipefail

if [ $# -ne 2 ]; then
  echo "usage: $0 <tag> <out dir>" >&2
  exit 2
fi
tag=$1
out=$2

version=$(cat pkg/build/version.txt)
if [ "$tag" != "$version" ]; then
  echo "tag $tag does not match pkg/build/version.txt ($version)" >&2
  exit 1
fi

pkg=github.com/cockroachdb/cockroach/pkg/build
built=$(TZ=UTC git log -1 --format=%cd --date=format-local:'%Y/%m/%d %H:%M:%S')
ldflags="-X $pkg.typ=release -X $pkg.rev=$(git rev-parse HEAD) -X '$pkg.utcTime=$built'"

rm -rf "$out"
mkdir -p "$out/image"
for arch in amd64 arm64; do
  name=patchroaddb-$tag.linux-$arch
  mkdir "$out/$name"
  CGO_ENABLED=0 GOOS=linux GOARCH=$arch go build -trimpath -ldflags "$ldflags" \
    -o "$out/$name/patchroaddb" ./pkg/cmd/patchroaddb
  cp -r LICENSE NOTICE licenses "$out/$name/"
  tar -czf "$out/$name.tgz" -C "$out" "$name"
  cp "$out/$name/patchroaddb" "$out/image/patchroaddb-$arch"
  rm -r "${out:?}/$name"
done
(cd "$out" && sha256sum patchroaddb-*.tgz >SHA256SUMS)

cp -r build/deploy/Dockerfile build/deploy/patchroaddb.sh LICENSE NOTICE licenses "$out/image/"
