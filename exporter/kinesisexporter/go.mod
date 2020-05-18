module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kinesisexporter

go 1.14

require (
	github.com/signalfx/opencensus-go-exporter-kinesis v0.4.2
	github.com/stretchr/testify v1.5.1
	go.opentelemetry.io/collector v0.3.1-0.20200518164231-3729dac06f74
	go.uber.org/zap v1.10.0
)

replace git.apache.org/thrift.git v0.12.0 => github.com/apache/thrift v0.12.0
