FROM golang:1.25.1-bookworm AS builder

WORKDIR /src
COPY . .

RUN GO111MODULE=on GOWORK=off go build -mod=vendor -o /out/lssh ./cmd/lssh && \
    GO111MODULE=on GOWORK=off go build -mod=vendor -o /out/lscp ./cmd/lscp && \
    GO111MODULE=on GOWORK=off go build -mod=vendor -o /out/lsftp ./cmd/lsftp && \
    GO111MODULE=on GOWORK=off go build -mod=vendor -o /out/lssync ./cmd/lssync && \
    GO111MODULE=on GOWORK=off go build -mod=vendor -o /out/lsdiff ./cmd/lsdiff && \
    GO111MODULE=on GOWORK=off go build -mod=vendor -o /out/lsshfs ./cmd/lsshfs && \
    GO111MODULE=on GOWORK=off go build -mod=vendor -o /out/lsshell ./cmd/lsshell && \
    GO111MODULE=on GOWORK=off go build -mod=vendor -o /out/lsmon ./cmd/lsmon && \
    GO111MODULE=on GOWORK=off go build -mod=vendor -o /out/lsmux ./cmd/lsmux

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash \
      ca-certificates \
      curl \
      fuse3 \
      gzip \
      iproute2 \
      iputils-ping \
      less \
      openssh-server \
      netcat-openbsd \
      openssh-client \
      procps \
      smbclient \
      sshpass \
      tmux \
      vim && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash demo

COPY --from=builder /out/lssh /usr/local/bin/lssh
COPY --from=builder /out/lscp /usr/local/bin/lscp
COPY --from=builder /out/lsftp /usr/local/bin/lsftp
COPY --from=builder /out/lssync /usr/local/bin/lssync
COPY --from=builder /out/lsdiff /usr/local/bin/lsdiff
COPY --from=builder /out/lsshfs /usr/local/bin/lsshfs
COPY --from=builder /out/lsshell /usr/local/bin/lsshell
COPY --from=builder /out/lsmon /usr/local/bin/lsmon
COPY --from=builder /out/lsmux /usr/local/bin/lsmux
COPY demo/client/entrypoint.sh /usr/local/bin/demo-client-entrypoint.sh
COPY demo/client/lssh-bastion.sh /usr/local/bin/demo-lssh-bastion.sh
COPY demo/client/setup-sync-fixtures.sh /usr/local/bin/demo-client-sync-fixtures.sh
COPY demo/client/home/ /home/demo/

RUN chown -R demo:demo /home/demo && \
    chmod 700 /home/demo/.ssh && \
    chmod 600 /home/demo/.ssh/demo_lssh_ed25519 /home/demo/.lssh.conf && \
    chmod +x /usr/local/bin/demo-client-entrypoint.sh /usr/local/bin/demo-lssh-bastion.sh /usr/local/bin/demo-client-sync-fixtures.sh && \
    /usr/local/bin/demo-client-sync-fixtures.sh && \
    chmod +x /home/demo/bin/demo-check.sh /home/demo/.demo_localrc/bin/update_lvim /home/demo/.demo_localrc/bin/update_ltmux

WORKDIR /home/demo
ENV HOME=/home/demo
EXPOSE 2222

ENTRYPOINT ["/usr/local/bin/demo-client-entrypoint.sh"]
CMD ["su", "-s", "/bin/bash", "demo", "-c", "sleep infinity"]
