module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver

go 1.17

require (
	github.com/apache/thrift v0.14.2
	github.com/gorilla/mux v1.8.0
	github.com/jaegertracing/jaeger v1.26.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.35.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.35.0
	github.com/stretchr/testify v1.7.0
	github.com/uber/jaeger-lib v2.4.1+incompatible
	go.opentelemetry.io/collector v0.35.0
	go.opentelemetry.io/collector/model v0.35.0
	go.uber.org/zap v1.19.0
	google.golang.org/grpc v1.40.0
)

require (
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.23.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.23.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger => ../../pkg/translator/jaeger
