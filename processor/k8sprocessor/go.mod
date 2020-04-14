module github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/open-telemetry/opentelemetry-collector v0.3.1-0.20200414190247-75ae9198a89e
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.3
	go.uber.org/zap v1.13.0
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
)
