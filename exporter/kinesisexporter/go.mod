module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kinesisexporter

go 1.12

require (
	github.com/open-telemetry/opentelemetry-collector v0.2.2
	github.com/signalfx/opencensus-go-exporter-kinesis v0.4.2
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.10.0
)

replace git.apache.org/thrift.git v0.12.0 => github.com/apache/thrift v0.12.0
