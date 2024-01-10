module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/examples/federation/prom-counter

go 1.20

require (
	github.com/prometheus/client_golang v1.18.0
	go.opentelemetry.io/otel v1.21.0
	go.opentelemetry.io/otel/exporters/prometheus v0.44.0
	go.opentelemetry.io/otel/metric v1.21.0
	go.opentelemetry.io/otel/sdk/metric v1.21.0
	go.uber.org/zap v1.26.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	go.opentelemetry.io/otel/sdk v1.21.0 // indirect
	go.opentelemetry.io/otel/trace v1.21.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
