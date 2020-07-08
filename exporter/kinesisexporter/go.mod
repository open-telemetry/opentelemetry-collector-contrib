module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kinesisexporter

go 1.14

require (
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/signalfx/opencensus-go-exporter-kinesis v0.4.2
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.5.1-0.20200708003418-541edde63b3a
	go.uber.org/zap v1.13.0
)

replace git.apache.org/thrift.git v0.12.0 => github.com/apache/thrift v0.12.0
