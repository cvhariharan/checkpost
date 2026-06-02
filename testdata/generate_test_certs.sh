#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cert_path="${script_dir}/server_cert.pem"
key_path="${script_dir}/server_key.pem"
config_path="$(mktemp)"

cleanup() {
  rm -f "${config_path}"
}
trap cleanup EXIT

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is required to generate test certificates" >&2
  exit 1
fi

cat >"${config_path}" <<'EOF'
[req]
default_bits = 2048
prompt = no
default_md = sha256
x509_extensions = v3_req
distinguished_name = dn

[dn]
CN = localhost

[v3_req]
subjectAltName = @alt_names
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
basicConstraints = critical, CA:FALSE

[alt_names]
DNS.1 = localhost
DNS.2 = checkpost
IP.1 = 127.0.0.1
EOF

openssl req \
  -x509 \
  -nodes \
  -days 3650 \
  -newkey rsa:2048 \
  -keyout "${key_path}" \
  -out "${cert_path}" \
  -config "${config_path}"

echo "Generated:"
echo "  ${cert_path}"
echo "  ${key_path}"
echo
echo "Certificate subject CN: localhost"
echo "Subject alternative names: localhost, checkpost, 127.0.0.1"
