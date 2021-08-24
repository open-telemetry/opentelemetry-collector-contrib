module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver

go 1.16

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.0.0-00010101000000-000000000000
	github.com/rs/cors v1.8.0
	github.com/soheilhy/cmux v0.1.5
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.33.1-0.20210820002854-d3000232f8f6
	go.opentelemetry.io/collector/model v0.33.1-0.20210820002854-d3000232f8f6
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.22.0
	go.opentelemetry.io/otel v1.0.0-RC2
	go.opentelemetry.io/otel/oteltest v1.0.0-RC2
	go.opentelemetry.io/otel/trace v1.0.0-RC2
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil => ../../internal/coreinternal/testutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver => ./

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../pkg/translator/opencensus

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter => ../../exporter/opencensusexporter
