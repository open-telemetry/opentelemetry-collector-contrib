
all: ca.crt envoy.crt tracing-client.crt otel-collector.crt 

clean:
	rm -rf *.key *.crt 
	rm -f *.key *.crt *.csr ca.srl

ca.key:
	openssl genrsa -out ca.key 2048

ca.crt: ca.key
	openssl req -x509 -new -nodes -key ca.key -sha256 -days 365 -out ca.crt -config ca.ext

envoy.key:
	openssl genrsa -out envoy.key 2048

envoy.csr: envoy.key
	openssl req -new -key envoy.key -out envoy.csr -config envoy.ext -extensions 'v3_req'

envoy.crt: envoy.csr
	openssl x509 -req -in envoy.csr -CA ca.crt -CAkey ca.key -out envoy.crt -CAcreateserial -days 365 -sha256 -extfile envoy.ext -extensions 'v3_req'

tracing-client.key:
	openssl genrsa -out tracing-client.key 2048

tracing-client.csr: tracing-client.key
	openssl req -new -key tracing-client.key -out tracing-client.csr -config tracing-client.ext -extensions 'v3_req'

tracing-client.crt: tracing-client.csr
	openssl x509 -req -in tracing-client.csr -CA ca.crt -CAkey ca.key -out tracing-client.crt -CAcreateserial -days 365 -sha256 -extfile tracing-client.ext -extensions 'v3_req'

otel-collector.key:
	openssl genrsa -out otel-collector.key 2048

otel-collector.csr: otel-collector.key
	openssl req -new -key otel-collector.key -out otel-collector.csr -config otel-collector.ext -extensions 'v3_req'

otel-collector.crt: otel-collector.csr
	openssl x509 -req -in otel-collector.csr -CA ca.crt -CAkey ca.key -out otel-collector.crt -CAcreateserial -days 365 -sha256 -extfile otel-collector.ext -extensions 'v3_req'
