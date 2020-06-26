module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/iancoleman/strcase v0.0.0-20171129010253-3de563c3dc08
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.4.0
	github.com/stretchr/testify v1.5.1
	github.com/uber-go/atomic v1.4.0 // indirect
	go.opentelemetry.io/collector v0.4.1-0.20200625162555-bd886e86b7ca
	go.uber.org/zap v1.14.1
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
