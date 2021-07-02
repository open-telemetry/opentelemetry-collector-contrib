module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter

go 1.16

require (
	github.com/aws/aws-sdk-go v1.38.71
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.2.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.1-0.20210701204331-d1fced9688ba
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.18.1
	google.golang.org/protobuf v1.27.1
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics => ./../../internal/aws/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ./../../internal/aws/awsutil

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210701204331-d1fced9688ba
