#!/usr/bin/env bash
set -euo pipefail

echo "[1/5] client should not reach over_proxy_ssh directly"
if nc -z -w 2 172.31.1.41 22; then
    echo "unexpected: direct access succeeded"
    exit 1
fi
echo "ok: direct access blocked"

echo "[2/5] password auth with OpenSSH"
sshpass -p demo-password ssh -o StrictHostKeyChecking=no demo@172.31.0.21 hostname

echo "[3/5] key auth with OpenSSH"
ssh -i ~/.ssh/demo_lssh_ed25519 demo@172.31.0.22 hostname

echo "[4/5] over ssh proxy with lssh"
lssh --host OverSshProxy hostname

echo "[5/5] over socks proxy with lssh"
lssh --host OverSocksProxy hostname
