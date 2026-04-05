#!/usr/bin/env bash
set -euo pipefail

EXTERNAL_ADDR="${SOCKS_EXTERNAL_ADDR:?SOCKS_EXTERNAL_ADDR is required}"
ALLOW_FROM="${SOCKS_ALLOW_FROM:?SOCKS_ALLOW_FROM is required}"

sed \
    -e "s/__SOCKS_EXTERNAL_ADDR__/${EXTERNAL_ADDR}/g" \
    -e "s#__SOCKS_ALLOW_FROM__#${ALLOW_FROM}#g" \
    /etc/danted.conf.template >/etc/danted.conf

exec /usr/sbin/danted -f /etc/danted.conf -N 1
