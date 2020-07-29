module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter

go 1.14

require (
	github.com/Azure/go-autorest/autorest/adal v0.9.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/signalfx/sapm-proto v0.5.3
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.5.1-0.20200728200651-9cbf43e372f0
	go.uber.org/zap v1.15.0
	google.golang.org/grpc/examples v0.0.0-20200723182653-9106c3fff523 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
