module github.com/open-telemetry/opentelemetry-collector-contrib/tracegen

go 1.16

require (
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/otel v1.0.0-RC2
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.0.0-RC2
	go.opentelemetry.io/otel/sdk v1.0.0-RC2
	go.opentelemetry.io/otel/trace v1.0.0-RC2
	go.uber.org/zap v1.18.1
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	google.golang.org/grpc v1.39.1
)
