#!/usr/bin/env bash
set -euo pipefail

cd /home/demo

if [[ -n "${SSH_ORIGINAL_COMMAND:-}" ]]; then
    read -r -a args <<<"${SSH_ORIGINAL_COMMAND}"
    exec /usr/local/bin/lssh "${args[@]}"
fi

if [[ "$#" -gt 0 ]]; then
    exec /usr/local/bin/lssh "$@"
fi

exec /usr/local/bin/lssh
