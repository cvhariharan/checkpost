# Fedora test client: installs osquery via Checkpost's bootstrap installer.
FROM fedora:latest

# Prerequisites to fetch and run the bootstrap installer.
RUN dnf install -y curl ca-certificates tar gzip && dnf clean all

COPY testdata/ca_cert.pem /etc/pki/ca-trust/source/anchors/checkpost-test-ca.pem
RUN update-ca-trust extract

RUN mkdir -p /etc/osquery-tls && cp /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem /etc/osquery-tls/server.pem
