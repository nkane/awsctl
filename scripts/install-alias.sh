#!/usr/bin/env bash
# Creates an `ac` symlink alongside an installed `awsctl` binary.
# Usage: ./scripts/install-alias.sh
set -euo pipefail

bin="$(command -v awsctl || true)"
if [ -z "$bin" ]; then
  echo "awsctl not found in PATH; install it first (go install github.com/nkane/awsctl/cmd/awsctl@latest)" >&2
  exit 1
fi

dir="$(dirname "$bin")"
ln -sf "$bin" "$dir/ac"
echo "linked $dir/ac -> $bin"
