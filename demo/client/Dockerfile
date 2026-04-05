FROM golang:1.25.1-bookworm AS builder

WORKDIR /src
COPY . .

RUN GO111MODULE=on go build -mod=vendor -o /out/lssh ./cmd/lssh && \
    GO111MODULE=on go build -mod=vendor -o /out/lscp ./cmd/lscp && \
    GO111MODULE=on go build -mod=vendor -o /out/lsftp ./cmd/lsftp && \
    GO111MODULE=on go build -mod=vendor -o /out/lsshell ./cmd/lsshell && \
    GO111MODULE=on go build -mod=vendor -o /out/lsmon ./cmd/lsmon

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash \
      ca-certificates \
      curl \
      gzip \
      iproute2 \
      iputils-ping \
      less \
      openssh-server \
      netcat-openbsd \
      openssh-client \
      procps \
      sshpass \
      tmux \
      vim && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash demo

COPY --from=builder /out/lssh /usr/local/bin/lssh
COPY --from=builder /out/lscp /usr/local/bin/lscp
COPY --from=builder /out/lsftp /usr/local/bin/lsftp
COPY --from=builder /out/lsshell /usr/local/bin/lsshell
COPY --from=builder /out/lsmon /usr/local/bin/lsmon
COPY demo/client/entrypoint.sh /usr/local/bin/demo-client-entrypoint.sh
COPY demo/client/lssh-bastion.sh /usr/local/bin/demo-lssh-bastion.sh
COPY demo/client/home/ /home/demo/

RUN chown -R demo:demo /home/demo && \
    chmod 700 /home/demo/.ssh && \
    chmod 600 /home/demo/.ssh/demo_lssh_ed25519 /home/demo/.lssh.conf && \
    chmod +x /usr/local/bin/demo-client-entrypoint.sh /usr/local/bin/demo-lssh-bastion.sh && \
    chmod +x /home/demo/bin/demo-check.sh /home/demo/.demo_localrc/bin/update_lvim /home/demo/.demo_localrc/bin/update_ltmux

USER demo
WORKDIR /home/demo
ENV HOME=/home/demo
EXPOSE 2222

ENTRYPOINT ["/usr/local/bin/demo-client-entrypoint.sh"]
CMD ["sleep", "infinity"]
