#!/usr/bin/env bash
set -euo pipefail

base="/etc/lssh-demo-sync"

mkdir -p "${base}"

cat >"${base}/README" <<'EOF'
Fixture templates copied into the demo user's home directory at container start.
EOF

mkdir -p "${base}/local-one-way/extra" \
         "${base}/bidirectional-remote/nested"

cat >"${base}/local-one-way/root.txt" <<'EOF'
old remote root
EOF
cat >"${base}/local-one-way/extra/remove-me.txt" <<'EOF'
delete me
EOF

cat >"${base}/bidirectional-remote/remote-only.txt" <<'EOF'
remote only file
EOF
cat >"${base}/bidirectional-remote/shared.txt" <<'EOF'
remote newer shared
EOF
cat >"${base}/bidirectional-remote/nested/remote-nested.txt" <<'EOF'
remote nested only
EOF

touch -t 202301010101 "${base}/local-one-way/root.txt" \
                     "${base}/local-one-way/extra/remove-me.txt"
touch -t 202401020202 "${base}/bidirectional-remote/remote-only.txt" \
                     "${base}/bidirectional-remote/shared.txt" \
                     "${base}/bidirectional-remote/nested/remote-nested.txt"
