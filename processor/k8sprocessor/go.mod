module github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor

go 1.13

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/open-telemetry/opentelemetry-collector v0.2.4-0.20200115225140-264426a9cae4
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.2
	go.uber.org/zap v1.13.0
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v12.0.0+incompatible
)
