module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter

go 1.14

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/elastic/go-elasticsearch/v7 v7.11.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.21.0
	go.uber.org/zap v1.16.0
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
)
