#!/usr/bin/env bash
set -euo pipefail

echo "[1/7] client should not reach over_proxy_ssh directly"
if nc -z -w 2 172.31.1.41 22; then
    echo "unexpected: direct access succeeded"
    exit 1
fi
echo "ok: direct access blocked"

echo "[2/7] password auth with OpenSSH"
sshpass -p demo-password ssh -o StrictHostKeyChecking=no demo@172.31.0.21 hostname

echo "[3/7] key auth with OpenSSH"
ssh -i ~/.ssh/demo_lssh_ed25519 demo@172.31.0.22 hostname

echo "[4/7] over ssh proxy with lssh"
lssh --host OverSshProxy hostname

echo "[5/7] over socks proxy with lssh"
lssh --host OverSocksProxy hostname

echo "[6/7] local_rc functions should be available over lssh"
lssh --host LocalRcKeyAuth 'type lvim >/dev/null && type ltmux >/dev/null && echo local_rc_ok'

echo "[7/7] generated vimrc wrapper should be usable on the remote host"
lssh --host LocalRcKeyAuth 'vim "+set nomore" "+set statusline?" "+q" | tail -n 1'
