module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/iancoleman/strcase v0.0.0-20171129010253-3de563c3dc08
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.12.1-0.20201016230751-46aada6e3c3a
	go.uber.org/atomic v1.7.0
	go.uber.org/zap v1.16.0
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig
