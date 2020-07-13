module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver

go 1.14

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/go-errors/errors v1.1.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.4.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver v0.5.0
	github.com/signalfx/golib v2.5.1+incompatible
	github.com/signalfx/golib/v3 v3.3.7
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.5.0
	go.uber.org/zap v1.15.0
)
