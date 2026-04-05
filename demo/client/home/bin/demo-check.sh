#!/usr/bin/env bash
set -euo pipefail

echo "[1/12] client bastion command should launch lssh"
grep -q '^ForceCommand /usr/local/bin/demo-lssh-bastion.sh$' ~/.demo-sshd/sshd_config
/usr/local/bin/demo-lssh-bastion.sh --list | grep -q OverNestedSocksProxy

echo "[2/12] client should not reach over_proxy_ssh directly"
if nc -z -w 2 172.31.1.41 22; then
    echo "unexpected: direct access succeeded"
    exit 1
fi
echo "ok: direct access blocked"

echo "[3/12] client should not reach deep_proxy_ssh directly"
if nc -z -w 2 172.31.2.51 22; then
    echo "unexpected: direct access to deep host succeeded"
    exit 1
fi
echo "ok: deep direct access blocked"

echo "[4/12] password auth with OpenSSH"
sshpass -p demo-password ssh -o StrictHostKeyChecking=no demo@172.31.0.21 hostname

echo "[5/12] key auth with OpenSSH"
ssh -i ~/.ssh/demo_lssh_ed25519 demo@172.31.0.22 hostname

echo "[6/12] over ssh proxy with lssh"
lssh --host OverSshProxy hostname

echo "[7/12] over nested ssh proxy with lssh"
lssh --host OverNestedSshProxy hostname

echo "[8/12] over nested http proxy with lssh"
lssh --host OverNestedHttpProxy hostname

echo "[9/12] over nested socks proxy with lssh"
lssh --host OverNestedSocksProxy hostname

echo "[10/12] over socks proxy with lssh"
lssh --host OverSocksProxy hostname

echo "[11/12] local_rc functions should be available over lssh"
lssh --host LocalRcKeyAuth 'type lvim >/dev/null && type ltmux >/dev/null && echo local_rc_ok'

echo "[12/12] generated vimrc wrapper should be loaded on the remote host"
lssh --host LocalRcKeyAuth 'declare -f lvim | grep -F "vim -u <(printf" && echo lvim_wrapper_ok'
