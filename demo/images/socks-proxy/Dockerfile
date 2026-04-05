FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      dante-server \
      vim && \
    rm -rf /var/lib/apt/lists/*

COPY demo/images/socks-proxy/danted.conf /etc/danted.conf

EXPOSE 1080

CMD ["/usr/sbin/danted", "-f", "/etc/danted.conf", "-N", "1"]
