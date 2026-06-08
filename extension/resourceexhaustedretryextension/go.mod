module github.com/open-telemetry/opentelemetry-collector-contrib/extension/resourceexhaustedretryextension

go 1.22.0

require (
	go.opentelemetry.io/collector/component v0.113.0
	go.opentelemetry.io/collector/extension v0.113.0
)

require (
	github.com/gogo/protobuf v1.3.2 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.113.0 // indirect
	go.opentelemetry.io/collector/pdata v1.19.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.72.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
