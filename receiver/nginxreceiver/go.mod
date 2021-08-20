module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver

go 1.16

require (
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/nginxinc/nginx-prometheus-exporter v0.8.1-0.20201110005315-f5a5f8086c19
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.11.1
	go.opentelemetry.io/collector v0.32.0
	go.opentelemetry.io/collector/model v0.32.0
	go.uber.org/zap v1.19.0
	golang.org/x/net v0.0.0-20210813160813-60bc85c4be6d // indirect
	golang.org/x/sys v0.0.0-20210820121016-41cdb8703e55 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20210820002220-43fce44e7af1 // indirect
	google.golang.org/grpc v1.40.0
)
