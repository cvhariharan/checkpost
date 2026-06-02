# Fedora test client: installs osquery via Checkpost's bootstrap installer.
FROM fedora:latest

# Prerequisites to fetch and run the bootstrap installer.
RUN dnf install -y curl ca-certificates && dnf clean all

# Trust the Checkpost test server certificate so the installer's HTTPS download validates.
# Build context is the repo root (see dev/docker-compose.yml).
COPY testdata/server_cert.pem /etc/pki/ca-trust/source/anchors/checkpost-test.pem
RUN update-ca-trust extract

# osquery ignores the system trust store, so also keep the cert at a fixed path the
# enrollment flags can point at via --tls_server_certs.
COPY testdata/server_cert.pem /etc/osquery-tls/server.pem
