#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ca_cert_path="${script_dir}/ca_cert.pem"
ca_key_path="${script_dir}/ca_key.pem"
cert_path="${script_dir}/server_cert.pem"
key_path="${script_dir}/server_key.pem"

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is required to generate test certificates" >&2
  exit 1
fi

workdir="$(mktemp -d)"
cleanup() {
  rm -rf "${workdir}"
}
trap cleanup EXIT

openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
  -keyout "${ca_key_path}" -out "${ca_cert_path}" \
  -subj "/CN=Checkpost Test CA" \
  -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
  -addext "keyUsage=critical,keyCertSign,cRLSign"

cat >"${workdir}/server.cnf" <<'EOF'
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn

[dn]
CN = localhost

[v3_ext]
subjectAltName = @alt_names
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
basicConstraints = critical, CA:FALSE

[alt_names]
DNS.1 = localhost
DNS.2 = checkpost
IP.1 = 127.0.0.1
EOF

openssl req -nodes -newkey rsa:2048 \
  -keyout "${key_path}" -out "${workdir}/server.csr" \
  -config "${workdir}/server.cnf"

openssl x509 -req -days 3650 \
  -in "${workdir}/server.csr" \
  -CA "${ca_cert_path}" -CAkey "${ca_key_path}" \
  -CAserial "${workdir}/ca.srl" -CAcreateserial \
  -extfile "${workdir}/server.cnf" -extensions v3_ext \
  -out "${cert_path}"

echo "Generated:"
echo "  ${ca_cert_path}      (CA; clients add this to their trust store)"
echo "  ${ca_key_path}       (CA private key; only needed to re-issue certs)"
echo "  ${cert_path}  (server leaf signed by the CA)"
echo "  ${key_path}   (server private key)"
echo
echo "Server certificate subject CN: localhost"
echo "Subject alternative names: localhost, checkpost, 127.0.0.1"
