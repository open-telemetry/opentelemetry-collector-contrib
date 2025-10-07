# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

# Generates scripts for integration test

days_validity=3650

if ! command -v openssl &> /dev/null
then
    echo "openssl could not be found"
    exit
fi

echo "Generating CA certificate..."
openssl genrsa -out CAroot.key 2048
openssl req -x509 -subj "/CN=clickhouse.local CA" -nodes -key CAroot.key -days $days_validity -out CAroot.crt

echo "Generating server certificate..."
cat > server_cert.conf << EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN=clickhouse

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = clickhouse
DNS.2 = localhost
DNS.3 = *.localhost
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

openssl req -newkey rsa:2048 -nodes -config server_cert.conf -keyout server.key -out server.csr
openssl x509 -req -in server.csr -out server.crt -CA CAroot.crt -CAkey CAroot.key -days $days_validity -extensions v3_req -extfile server_cert.conf

echo "Generating client certificate..."
openssl req -newkey rsa:2048 -nodes -subj "/CN=client" -keyout client.key -out client.csr
openssl x509 -req -in client.csr -out client.crt -CA CAroot.crt -CAkey CAroot.key -days $days_validity

rm server.csr client.csr server_cert.conf

chmod 644 *.crt *.key

echo "Certificates generated successfully!"
echo "Files created:"
echo "  - CAroot.crt (CA certificate)"
echo "  - CAroot.key (CA private key)"
echo "  - server.crt (Server certificate)"
echo "  - server.key (Server private key)"
echo "  - client.crt (Client certificate)"
echo "  - client.key (Client private key)"

echo ""
echo "Server certificate details:"
openssl x509 -in server.crt -text -noout | grep -A 1 "Subject Alternative Name"
