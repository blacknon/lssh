FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      dante-server \
      bash \
      vim && \
    rm -rf /var/lib/apt/lists/*

COPY demo/images/socks-proxy/danted.conf /etc/danted.conf.template
COPY demo/images/socks-proxy/entrypoint.sh /usr/local/bin/demo-socks-proxy-entrypoint.sh

RUN chmod +x /usr/local/bin/demo-socks-proxy-entrypoint.sh

EXPOSE 1080

CMD ["/usr/local/bin/demo-socks-proxy-entrypoint.sh"]
