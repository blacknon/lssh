#!/usr/bin/env bash
set -euo pipefail

SSH_USER="${SSH_USER:-demo}"
SSH_PASSWORD="${SSH_PASSWORD:-demo-password}"
ENABLE_PASSWORD_AUTH="${ENABLE_PASSWORD_AUTH:-false}"
ENABLE_PUBKEY_AUTH="${ENABLE_PUBKEY_AUTH:-true}"
AUTHORIZED_KEY="${AUTHORIZED_KEY:-}"

password_auth_sshd=no
pubkey_auth_sshd=no

if [[ "${ENABLE_PASSWORD_AUTH}" == "true" ]]; then
    password_auth_sshd=yes
fi

if [[ "${ENABLE_PUBKEY_AUTH}" == "true" ]]; then
    pubkey_auth_sshd=yes
fi

if ! id "${SSH_USER}" >/dev/null 2>&1; then
    useradd -m -s /bin/bash "${SSH_USER}"
fi

# Set a password even for key-only users so the account is unlocked.
# PasswordAuthentication still controls whether SSH password login is allowed.
echo "${SSH_USER}:${SSH_PASSWORD}" | chpasswd

install -d -m 755 -o "${SSH_USER}" -g "${SSH_USER}" "/home/${SSH_USER}/demo-sync"
rm -rf "/home/${SSH_USER}/demo-sync/local-one-way" "/home/${SSH_USER}/demo-sync/bidirectional-remote"
cp -a /etc/lssh-demo-sync/local-one-way "/home/${SSH_USER}/demo-sync/local-one-way"
cp -a /etc/lssh-demo-sync/bidirectional-remote "/home/${SSH_USER}/demo-sync/bidirectional-remote"
chown -R "${SSH_USER}:${SSH_USER}" "/home/${SSH_USER}/demo-sync"

if [[ "${ENABLE_PUBKEY_AUTH}" == "true" && -n "${AUTHORIZED_KEY}" ]]; then
    install -d -m 700 -o "${SSH_USER}" -g "${SSH_USER}" "/home/${SSH_USER}/.ssh"
    printf '%s\n' "${AUTHORIZED_KEY}" >"/home/${SSH_USER}/.ssh/authorized_keys"
    chown "${SSH_USER}:${SSH_USER}" "/home/${SSH_USER}/.ssh/authorized_keys"
    chmod 600 "/home/${SSH_USER}/.ssh/authorized_keys"
fi

cat >/etc/ssh/sshd_config.d/demo.conf <<EOF
Port 22
PasswordAuthentication ${password_auth_sshd}
PubkeyAuthentication ${pubkey_auth_sshd}
KbdInteractiveAuthentication no
ChallengeResponseAuthentication no
UsePAM no
PermitRootLogin no
AllowUsers ${SSH_USER}
AllowAgentForwarding yes
AllowTcpForwarding yes
GatewayPorts yes
X11Forwarding yes
AuthorizedKeysFile .ssh/authorized_keys
PrintMotd no
EOF

ssh-keygen -A >/dev/null

exec /usr/sbin/sshd -D -e
