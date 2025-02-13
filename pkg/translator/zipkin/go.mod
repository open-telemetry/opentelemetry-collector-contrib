module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin

go 1.23.0

require (
	github.com/apache/thrift v0.21.0
	github.com/gogo/protobuf v1.3.2
	github.com/jaegertracing/jaeger-idl v0.5.0
	github.com/kr/pretty v0.3.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.119.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils v0.119.0
	github.com/openzipkin/zipkin-go v0.4.3
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/pdata v1.25.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/semconv v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/otel/trace v1.34.0
	go.uber.org/goleak v1.3.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.70.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils => ../../../pkg/core/xidutils

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../golden
