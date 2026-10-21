#!/usr/bin/env bash
set -euo pipefail
cd "${1:?Usage: verify-release-assets.sh ARTIFACT_DIRECTORY}"
shopt -s nullglob
files=(*)
[ "${#files[@]}" -eq 16 ] || {
  echo "Expected 8 archives and 8 checksums; found ${#files[@]} files." >&2
  exit 1
}
for platform in amd64 arm64 armv7 armv6 386 armv5 s390x windows-amd64; do
  if [ "$platform" = windows-amd64 ]; then
    archive="x-ui-windows-amd64.zip"
  else
    archive="x-ui-linux-$platform.tar.gz"
  fi
  [ -s "$archive" ] && [ -s "$archive.sha256" ] || {
    echo "Missing package or checksum: $archive" >&2
    exit 1
  }
  sha256sum -c "$archive.sha256"
  if [ "$platform" = windows-amd64 ]; then
    unzip -tq "$archive"
  else
    tar -tzf "$archive" >/dev/null
  fi
done
