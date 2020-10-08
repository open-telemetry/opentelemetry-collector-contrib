module github.com/open-telemetry/opentelemetry-collector-contrib/tracegen

go 1.14

require (
	github.com/go-logr/logr v0.2.1
	github.com/go-logr/zapr v0.2.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/otel v0.12.0
	go.opentelemetry.io/otel/exporters/otlp v0.12.0
	go.opentelemetry.io/otel/sdk v0.12.0
	go.uber.org/zap v1.16.0
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	google.golang.org/grpc v1.32.0
)
