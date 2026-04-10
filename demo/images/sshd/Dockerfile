FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash \
      ca-certificates \
      gzip \
      netcat-openbsd \
      openssh-server \
      procps \
      tmux \
      vim && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /var/run/sshd /etc/ssh/sshd_config.d

COPY demo/images/sshd/entrypoint.sh /usr/local/bin/demo-sshd-entrypoint.sh
COPY demo/images/sshd/setup-sync-fixtures.sh /usr/local/bin/demo-sshd-sync-fixtures.sh

RUN chmod +x /usr/local/bin/demo-sshd-entrypoint.sh /usr/local/bin/demo-sshd-sync-fixtures.sh && \
    /usr/local/bin/demo-sshd-sync-fixtures.sh

EXPOSE 22

ENTRYPOINT ["/usr/local/bin/demo-sshd-entrypoint.sh"]
