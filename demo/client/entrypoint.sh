#!/usr/bin/env bash
set -euo pipefail

HOME_DIR="/home/demo"
SSH_DIR="${HOME_DIR}/.ssh"
STATE_DIR="${HOME_DIR}/.demo-sshd"
HOST_KEY="${SSH_DIR}/demo_client_host_ed25519_key"
AUTHORIZED_KEYS="${SSH_DIR}/authorized_keys"
PUBKEY_FILE="${SSH_DIR}/demo_lssh_ed25519.pub"

mkdir -p /run/sshd
mkdir -p "${STATE_DIR}" "${SSH_DIR}"
chmod 700 "${SSH_DIR}"
chown -R demo:demo "${HOME_DIR}"

if [[ ! -f "${HOST_KEY}" ]]; then
    ssh-keygen -t ed25519 -f "${HOST_KEY}" -N "" >/dev/null
fi

if [[ -f "${PUBKEY_FILE}" ]]; then
    cat "${PUBKEY_FILE}" >"${AUTHORIZED_KEYS}"
    chown demo:demo "${AUTHORIZED_KEYS}" "${PUBKEY_FILE}" "${HOST_KEY}" "${HOST_KEY}.pub"
    chmod 600 "${AUTHORIZED_KEYS}"
fi

cat >"${STATE_DIR}/sshd_config" <<EOF
Port 2222
ListenAddress 0.0.0.0
PasswordAuthentication no
PubkeyAuthentication yes
KbdInteractiveAuthentication no
ChallengeResponseAuthentication no
UsePAM no
AllowUsers demo
ForceCommand /usr/local/bin/demo-lssh-bastion.sh
PidFile ${STATE_DIR}/sshd.pid
AuthorizedKeysFile ${AUTHORIZED_KEYS}
HostKey ${HOST_KEY}
PrintMotd no
Subsystem sftp internal-sftp
EOF

/usr/sbin/sshd -f "${STATE_DIR}/sshd_config" -E "${STATE_DIR}/sshd.log"

exec "$@"
