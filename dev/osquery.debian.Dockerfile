# Debian test client: installs osquery via Checkpost's bootstrap installer.
FROM debian:stable-slim

# Prerequisites to fetch and run the bootstrap installer.
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl ca-certificates tar gzip \
    && rm -rf /var/lib/apt/lists/*

# Trust the Checkpost test server certificate so the installer's HTTPS download validates.
# Build context is the repo root (see dev/docker-compose.yml).
COPY testdata/server_cert.pem /usr/local/share/ca-certificates/checkpost-test.crt
RUN update-ca-certificates

# osquery ignores the system trust store, so also keep the cert at a fixed path the
# enrollment flags can point at via --tls_server_certs.
COPY testdata/server_cert.pem /etc/osquery-tls/server.pem
