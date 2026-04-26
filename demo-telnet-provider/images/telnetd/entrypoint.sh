#!/usr/bin/env bash
set -euo pipefail

TELNET_USER="${TELNET_USER:-demo}"
TELNET_PASSWORD="${TELNET_PASSWORD:-demo-password}"
TELNET_HOST_ALIAS="${TELNET_HOST_ALIAS:-telnet-host}"

if ! id "${TELNET_USER}" >/dev/null 2>&1; then
    useradd -m -s /bin/bash "${TELNET_USER}"
fi

echo "${TELNET_USER}:${TELNET_PASSWORD}" | chpasswd
printf '%s\n' "${TELNET_HOST_ALIAS}" >/etc/hostname

# Some container runtimes do not allow changing the kernel hostname here.
# Keep the alias in /etc/hostname and continue even if the syscall is denied.
hostname "${TELNET_HOST_ALIAS}" >/dev/null 2>&1 || true

cat >/etc/issue <<EOF
${TELNET_HOST_ALIAS}
EOF

exec /usr/local/bin/demo-telnet-shell
