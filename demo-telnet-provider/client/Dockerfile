FROM golang:1.25.1-bookworm AS builder

WORKDIR /src
COPY . .

RUN GO111MODULE=on go build -mod=vendor -o /out/lssh ./cmd/lssh && \
    GO111MODULE=on go build -mod=vendor -o /out/provider-connector-telnet ./provider/connector/provider-connector-telnet

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash \
      ca-certificates \
      less \
      openssh-client \
      procps \
      vim && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash demo

COPY --from=builder /out/lssh /usr/local/bin/lssh
COPY --from=builder /out/provider-connector-telnet /home/demo/.config/lssh/providers/provider-connector-telnet
COPY demo-telnet-provider/client/home/ /home/demo/

RUN chown -R demo:demo /home/demo && \
    chmod 700 /home/demo/.ssh /home/demo/.config /home/demo/.config/lssh /home/demo/.config/lssh/providers && \
    chmod 600 /home/demo/.ssh/demo_lssh_ed25519 /home/demo/.lssh.conf

WORKDIR /home/demo
ENV HOME=/home/demo

CMD ["su", "-s", "/bin/bash", "demo", "-c", "sleep infinity"]
