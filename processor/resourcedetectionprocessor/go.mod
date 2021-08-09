module github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor

go 1.16

require (
	cloud.google.com/go v0.90.0
	github.com/Showmax/go-fqdn v1.0.0
	github.com/aws/aws-sdk-go v1.40.14
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/docker/docker v20.10.8+incompatible
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.31.1-0.20210809153342-28acc7d8b7f2
	go.opentelemetry.io/collector/model v0.31.1-0.20210809153342-28acc7d8b7f2
	go.uber.org/zap v1.18.1
)
