module github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../../pkg/pdatautil

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/collector/pdata v1.0.2-0.20240125183026-3cacd40b27e8
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.18.0 // indirect
	golang.org/x/sys v0.14.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231002182017-d307bd883b97 // indirect
	google.golang.org/grpc v1.60.1 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
)

go 1.20
