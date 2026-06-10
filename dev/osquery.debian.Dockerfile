# Debian test client: installs osquery via Checkpost's bootstrap installer.
FROM debian:stable-slim

# Prerequisites to fetch and run the bootstrap installer.
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl ca-certificates tar gzip \
    && rm -rf /var/lib/apt/lists/*

COPY testdata/ca_cert.pem /usr/local/share/ca-certificates/checkpost-test-ca.crt
RUN update-ca-certificates

RUN mkdir -p /etc/osquery-tls && cp /etc/ssl/certs/ca-certificates.crt /etc/osquery-tls/server.pem
