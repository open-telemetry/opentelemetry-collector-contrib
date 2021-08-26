module github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor

go 1.16

require (
	cloud.google.com/go v0.92.3
	github.com/Showmax/go-fqdn v1.0.0
	github.com/aws/aws-sdk-go v1.40.27
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/docker/docker v20.10.8+incompatible
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.33.1-0.20210826200354-479f46434f9a
	go.opentelemetry.io/collector/model v0.33.1-0.20210826200354-479f46434f9a
	go.uber.org/zap v1.19.0

)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../pkg/translator/opencensus

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal
