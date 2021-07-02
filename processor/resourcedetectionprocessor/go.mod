module github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor

go 1.16

require (
	cloud.google.com/go v0.84.0
	github.com/Showmax/go-fqdn v1.0.0
	github.com/aws/aws-sdk-go v1.38.69
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/docker/docker v20.10.7+incompatible
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.1-0.20210702000714-32c2d0f13167
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.18.1
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210702000714-32c2d0f13167
