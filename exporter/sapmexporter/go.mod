module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter

go 1.14

require (
	github.com/gopherjs/gopherjs v0.0.0-20181103185306-d547d1d9531e // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.4.0
	github.com/signalfx/sapm-proto v0.5.3
	github.com/smartystreets/assertions v0.0.0-20190215210624-980c5ac6f3ac // indirect
	github.com/stretchr/testify v1.5.1
	github.com/uber-go/atomic v1.4.0 // indirect
	go.opentelemetry.io/collector v0.4.1-0.20200625162555-bd886e86b7ca
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.13.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
