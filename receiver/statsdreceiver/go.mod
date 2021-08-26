module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver

go 1.16

require (
	github.com/montanaflynn/stats v0.0.0-20171201202039-1bf9dbcd8cbe
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.33.1-0.20210826200354-479f46434f9a
	go.opentelemetry.io/collector/model v0.33.1-0.20210826200354-479f46434f9a
	go.opentelemetry.io/otel v1.0.0-RC2
	go.uber.org/zap v1.19.0

)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal
