#!/usr/bin/env bash
set -euo pipefail

if [[ $# -eq 0 ]]; then
    echo "Usage: $0 <source-directory>" >&2
    exit 1
fi

src="$1"
timestamp=$(date +%Y%m%d-%H%M%S)
archive="backup-${timestamp}.tar.gz"

tar -czf "$archive" "$src"
echo "$archive"
