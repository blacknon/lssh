FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      squid \
      vim && \
    rm -rf /var/lib/apt/lists/*

COPY demo/images/http-proxy/squid.conf /etc/squid/squid.conf

EXPOSE 8888

CMD ["squid", "-N", "-f", "/etc/squid/squid.conf"]
