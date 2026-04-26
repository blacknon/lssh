FROM golang:1.25.1-bookworm AS builder

WORKDIR /src
COPY . .

RUN GO111MODULE=on go build -mod=vendor -o /out/demo-telnet-shell ./demo-telnet-provider/images/telnetd/server

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash \
      passwd \
      procps && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/demo-telnet-shell /usr/local/bin/demo-telnet-shell
COPY demo-telnet-provider/images/telnetd/entrypoint.sh /usr/local/bin/demo-telnetd-entrypoint.sh

RUN chmod +x /usr/local/bin/demo-telnetd-entrypoint.sh /usr/local/bin/demo-telnet-shell

EXPOSE 23

ENTRYPOINT ["/usr/local/bin/demo-telnetd-entrypoint.sh"]
