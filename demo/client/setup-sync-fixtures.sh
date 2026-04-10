#!/usr/bin/env bash
set -euo pipefail

base="/home/demo/.demo_sync"

mkdir -p "${base}/local-one-way/nested" \
         "${base}/bidirectional-local/nested"

cat >"${base}/local-one-way/root.txt" <<'EOF'
local one-way root
EOF
cat >"${base}/local-one-way/nested/child.txt" <<'EOF'
local nested child
EOF

cat >"${base}/bidirectional-local/local-only.txt" <<'EOF'
local only file
EOF
cat >"${base}/bidirectional-local/shared.txt" <<'EOF'
local old shared
EOF
cat >"${base}/bidirectional-local/nested/local-nested.txt" <<'EOF'
local nested only
EOF

touch -t 202401010101 "${base}/local-one-way/root.txt" \
                     "${base}/local-one-way/nested/child.txt" \
                     "${base}/bidirectional-local/local-only.txt" \
                     "${base}/bidirectional-local/shared.txt" \
                     "${base}/bidirectional-local/nested/local-nested.txt"

chown -R demo:demo "${base}"
