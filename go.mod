module github.com/open-telemetry/opentelemetry-collector-contrib

// NOTE:
// This go.mod is NOT used to build any official binary.
// To see the builder manifests used for official binaries,
// check https://github.com/open-telemetry/opentelemetry-collector-releases
//
// For the OpenTelemetry Collector Contrib distribution specifically, see
// https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib

go 1.24

retract (
	v0.76.2
	v0.76.1
	v0.65.0
	v0.37.0 // Contains dependencies on v0.36.0 components, which should have been updated to v0.37.0.
)

require (
	github.com/golang/snappy v1.0.0
	github.com/prometheus/prometheus v0.305.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component/componenttest v0.132.1-0.20250822050819-711177175dd4
	go.opentelemetry.io/collector/connector/connectortest v0.132.1-0.20250822050819-711177175dd4
	go.opentelemetry.io/collector/consumer v1.38.1-0.20250822050819-711177175dd4
	go.opentelemetry.io/collector/consumer/consumertest v0.132.1-0.20250822050819-711177175dd4
	go.opentelemetry.io/collector/pdata v1.38.1-0.20250822050819-711177175dd4
	go.opentelemetry.io/otel v1.37.0
	go.opentelemetry.io/otel/metric v1.37.0
	go.opentelemetry.io/otel/sdk/metric v1.37.0
	go.opentelemetry.io/otel/trace v1.37.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.132.1-0.20250822050819-711177175dd4 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.132.1-0.20250822050819-711177175dd4 // indirect
	go.opentelemetry.io/collector/featuregate v1.38.1-0.20250822050819-711177175dd4 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.132.1-0.20250822050819-711177175dd4 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.132.1-0.20250822050819-711177175dd4 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.132.1-0.20250822050819-711177175dd4 // indirect
	go.opentelemetry.io/collector/pipeline v1.38.1-0.20250822050819-711177175dd4 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.132.1-0.20250822050819-711177175dd4 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.12.0 // indirect
	go.opentelemetry.io/otel/log v0.13.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	github.com/gogo/protobuf v1.3.2
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	go.opentelemetry.io/collector/component v1.38.1-0.20250822050819-711177175dd4
	go.opentelemetry.io/collector/connector v0.132.1-0.20250822050819-711177175dd4
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	google.golang.org/grpc v1.75.0 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
)
