module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter

go 1.12

require (
	github.com/open-telemetry/opentelemetry-collector v0.2.1-0.20191119152140-567e1046cefa
	github.com/pierrec/lz4 v2.0.5+incompatible // indirect
	github.com/signalfx/sapm-proto v0.0.3-0.20191211161027-5e20f16d64d6
	github.com/stretchr/testify v1.4.0
	go.uber.org/atomic v1.5.1 // indirect
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.13.0
	google.golang.org/grpc v1.23.1 // indirect
)
