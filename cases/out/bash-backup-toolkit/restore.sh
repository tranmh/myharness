#!/usr/bin/env bash
set -euo pipefail

if [[ $# -eq 0 ]]; then
    echo "Usage: $0 <backup-*.tar.gz> [target-directory]" >&2
    exit 1
fi

archive="$1"
target="${2:-.}"

mkdir -p "$target"
tar -xzf "$archive" -C "$target"
